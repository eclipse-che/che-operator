//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package che

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/dashboard"
	devworkspace "github.com/eclipse-che/che-operator/pkg/deploy/dev-workspace"
	"github.com/eclipse-che/che-operator/pkg/deploy/devfileregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/deploy/pluginregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/postgres"
	"github.com/eclipse-che/che-operator/pkg/deploy/server"

	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	userv1 "github.com/openshift/api/user/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
)

var (
	// CheServiceAccountName - service account name for che-server.
	CheServiceAccountName = "che"

	failedSyncItemName = ""
	syncItems          = []func(*deploy.DeployContext) (bool, error){
		validateCheCR,
		readProxyConfiguration,
		syncOpenShiftCertificates,
		syncSelfSignedCertificate,
		deploy.SyncAdditionalCACertsConfigMapToCluster,
		syncImagePuller,
		deleteOAuthInitialUserIfNeeded,
		syncOpenShiftOAuth,
		devworkspace.ReconcileDevWorkspace,
		syncServiceAccount,
		reconcileGatewayPermissions,
		reconcileWorkspacePermissions,
		syncCheClusterRoles,
		syncWorkspaceClusterRoles,
		syncDefaults,
		syncPostgres,
		syncServerEndpoint,
		syncIdentityProvider,
		syncPluginRegistry,
		syncDevfileRegistry,
		syncDashboard,
		syncGateway,
		syncServer,
		deploy.ReconcileConsoleLink,
		reconcileIdentityProvider,
	}
)

const (
	failedValidationReason            = "InstallOrUpdateFailed"
	failedNoOpenshiftUser             = "NoOpenshiftUsers"
	failedNoIdentityProviders         = "NoIdentityProviders"
	failedUnableToGetOAuth            = "UnableToGetOpenshiftOAuth"
	warningNoIdentityProvidersMessage = "No Openshift identity providers."

	AddIdentityProviderMessage      = "Openshift oAuth was disabled. How to add identity provider read in the Help Link:"
	warningNoRealUsersMessage       = "No real users. Openshift oAuth was disabled. How to add new user read in the Help Link:"
	failedUnableToGetOpenshiftUsers = "Unable to get users on the OpenShift cluster."

	howToAddIdentityProviderLinkOS4 = "https://docs.openshift.com/container-platform/latest/authentication/understanding-identity-provider.html#identity-provider-overview_understanding-identity-provider"
	howToConfigureOAuthLinkOS3      = "https://docs.openshift.com/container-platform/3.11/install_config/configuring_authentication.html"
)

// CheClusterReconciler reconciles a CheCluster object
type CheClusterReconciler struct {
	Log    logr.Logger
	Scheme *k8sruntime.Scheme

	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client

	// This client, is a simple client
	// that reads objects without using the cache,
	// to simply read objects thta we don't intend
	// to further watch
	nonCachedClient client.Client
	// A discovery client to check for the existence of certain APIs registered
	// in the API Server
	discoveryClient   discovery.DiscoveryInterface
	tests             bool
	userHandler       OpenShiftOAuthUserHandler
	permissionChecker PermissionChecker
}

// NewReconciler returns a new CheClusterReconciler
func NewReconciler(mgr controller.Manager) (*CheClusterReconciler, error) {
	noncachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	return &CheClusterReconciler{
		Scheme: mgr.GetScheme(),
		Log:    controller.Log.WithName("controllers").WithName("CheCluster"),

		client:            mgr.GetClient(),
		nonCachedClient:   noncachedClient,
		discoveryClient:   discoveryClient,
		userHandler:       NewOpenShiftOAuthUserHandler(noncachedClient),
		permissionChecker: &K8sApiPermissionChecker{},
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheClusterReconciler) SetupWithManager(mgr controller.Manager) error {
	isOpenShift := util.IsOpenShift

	onAllExceptGenericEventsPredicate := predicate.Funcs{
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return true
		},
		CreateFunc: func(evt event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(evt event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(evt event.GenericEvent) bool {
			return false
		},
	}

	var toTrustedBundleConfigMapRequestMapper handler.ToRequestsFunc = func(obj handler.MapObject) []controller.Request {
		isTrusted, reconcileRequest := isTrustedBundleConfigMap(mgr, obj)
		if isTrusted {
			return []controller.Request{reconcileRequest}
		}
		return []controller.Request{}
	}

	var toEclipseCheRelatedObjRequestMapper handler.ToRequestsFunc = func(obj handler.MapObject) []controller.Request {
		isEclipseCheRelatedObj, reconcileRequest := isEclipseCheRelatedObj(mgr, obj)
		if isEclipseCheRelatedObj {
			return []controller.Request{reconcileRequest}
		}
		return []controller.Request{}
	}

	contollerBuilder := controller.NewControllerManagedBy(mgr).
		// Watch for changes to primary resource CheCluster
		Watches(&source.Kind{Type: &orgv1.CheCluster{}}, &handler.EnqueueRequestForObject{}).
		// Watch for changes to secondary resources and requeue the owner CheCluster
		Watches(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &rbac.Role{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &rbac.RoleBinding{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: toTrustedBundleConfigMapRequestMapper},
			builder.WithPredicates(onAllExceptGenericEventsPredicate),
		).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: toEclipseCheRelatedObjRequestMapper},
			builder.WithPredicates(onAllExceptGenericEventsPredicate),
		).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: toEclipseCheRelatedObjRequestMapper},
			builder.WithPredicates(onAllExceptGenericEventsPredicate),
		)

	if isOpenShift {
		contollerBuilder = contollerBuilder.Watches(&source.Kind{Type: &routev1.Route{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		})
	} else {
		contollerBuilder = contollerBuilder.Watches(&source.Kind{Type: &v1beta1.Ingress{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		})
	}

	return contollerBuilder.
		For(&orgv1.CheCluster{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.3/pkg/reconcile
func (r *CheClusterReconciler) Reconcile(req controller.Request) (controller.Result, error) {
	_ = r.Log.WithValues("checluster", req.NamespacedName)

	clusterAPI := deploy.ClusterAPI{
		Client:          r.client,
		NonCachedClient: r.nonCachedClient,
		DiscoveryClient: r.discoveryClient,
		Scheme:          r.Scheme,
	}

	// Fetch the CheCluster instance
	instance, err := r.GetCR(req)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return controller.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return controller.Result{}, err
	}

	deployContext := &deploy.DeployContext{
		ClusterAPI: clusterAPI,
		CheCluster: instance,
	}

	// Reconcile finalizers before CR is deleted
	r.reconcileFinalizers(deployContext)

	for _, syncItem := range syncItems {
		done, err := syncItem(deployContext)
		if !util.IsTestMode() {
			syncItemName := runtime.FuncForPC(reflect.ValueOf(syncItem).Pointer()).Name()
			if err != nil {
				failedSyncItemName = syncItemName
				logrus.Errorf("Failed to sync %s, cause: %v", failedSyncItemName, err)

				// update the status with the error message'
				if err := deploy.SetStatusDetails(deployContext, failedValidationReason, err.Error(), ""); err != nil {
					return controller.Result{RequeueAfter: time.Second}, err
				}
			} else if failedSyncItemName == syncItemName { // status must cleaned by the same item
				failedSyncItemName = ""
				if err := deploy.SetStatusDetails(deployContext, "", "", ""); err != nil {
					return controller.Result{RequeueAfter: time.Second}, err
				}
			}

			if !done {
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
		}
	}

	logrus.Info("Successfully reconciled.")
	return reconcile.Result{}, nil
}

// isTrustedBundleConfigMap detects whether given config map is the config map with additional CA certificates to be trusted by Che
func isTrustedBundleConfigMap(mgr controller.Manager, obj handler.MapObject) (bool, controller.Request) {
	checlusters := &orgv1.CheClusterList{}
	if err := mgr.GetClient().List(context.TODO(), checlusters, &client.ListOptions{}); err != nil {
		return false, controller.Request{}
	}

	if len(checlusters.Items) != 1 {
		return false, controller.Request{}
	}

	// Check if config map is the config map from CR
	if checlusters.Items[0].Spec.Server.ServerTrustStoreConfigMapName != obj.Meta.GetName() {
		// No, it is not form CR
		// Check for labels

		// Check for part of Che label
		if value, exists := obj.Meta.GetLabels()[deploy.KubernetesPartOfLabelKey]; !exists || value != deploy.CheEclipseOrg {
			// Labels do not match
			return false, controller.Request{}
		}

		// Check for CA bundle label
		if value, exists := obj.Meta.GetLabels()[deploy.CheCACertsConfigMapLabelKey]; !exists || value != deploy.CheCACertsConfigMapLabelValue {
			// Labels do not match
			return false, controller.Request{}
		}
	}

	return true, controller.Request{
		NamespacedName: types.NamespacedName{
			Namespace: checlusters.Items[0].Namespace,
			Name:      checlusters.Items[0].Name,
		},
	}
}

func syncOpenShiftOAuth(deployContext *deploy.DeployContext) (bool, error) {
	if !util.IsOpenShift || deployContext.CheCluster.Spec.Auth.OpenShiftoAuth != nil {
		return true, nil
	}

	oauth := false

	if util.IsOpenShift4 {
		openshitOAuth, err := GetOpenshiftOAuth(deployContext.ClusterAPI.NonCachedClient)
		if err != nil {
			logrus.Error("Unable to get Openshift oAuth. Cause: " + err.Error())
		} else {
			if len(openshitOAuth.Spec.IdentityProviders) > 0 {
				oauth = true
			} else if util.IsInitialOpenShiftOAuthUserEnabled(deployContext.CheCluster) {
				userHandler := NewOpenShiftOAuthUserHandler(deployContext.ClusterAPI.NonCachedClient)
				provisioned, err := userHandler.SyncOAuthInitialUser(openshitOAuth, deployContext)
				if err != nil {
					logrus.Error(warningNoIdentityProvidersMessage + " Operator tried to create initial OpenShift OAuth user for HTPasswd identity provider, but failed. Cause: " + err.Error())
					logrus.Info("To enable OpenShift OAuth, please add identity provider first: " + howToAddIdentityProviderLinkOS4)

					// Don't try to create initial user any more, che-operator shouldn't hang on this step.
					deployContext.CheCluster.Spec.Auth.InitialOpenShiftOAuthUser = nil
					if err := deploy.UpdateCheCRStatus(deployContext, "initialOpenShiftOAuthUser", ""); err != nil {
						return false, err
					}
					oauth = false
				} else {
					if !provisioned {
						return false, nil
					}
					oauth = true
					if deployContext.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret == "" {
						deployContext.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret = openShiftOAuthUserCredentialsSecret
						if err := deploy.UpdateCheCRStatus(deployContext, "openShiftOAuthUserCredentialsSecret", openShiftOAuthUserCredentialsSecret); err != nil {
							return false, err
						}
					}
				}
			}
		}
	} else { // Openshift 3
		users := &userv1.UserList{}
		listOptions := &client.ListOptions{}
		if err := deployContext.ClusterAPI.NonCachedClient.List(context.TODO(), users, listOptions); err != nil {
			logrus.Error(failedUnableToGetOpenshiftUsers + " Cause: " + err.Error())
		} else {
			oauth = len(users.Items) >= 1
			if !oauth {
				logrus.Warn(warningNoRealUsersMessage + " " + howToConfigureOAuthLinkOS3)
			}
		}
	}

	newOAuthValue := util.NewBoolPointer(oauth)
	if !reflect.DeepEqual(newOAuthValue, deployContext.CheCluster.Spec.Auth.OpenShiftoAuth) {
		deployContext.CheCluster.Spec.Auth.OpenShiftoAuth = newOAuthValue
		if err := deploy.UpdateCheCRSpec(deployContext, "openShiftoAuth", strconv.FormatBool(oauth)); err != nil {
			return false, err
		}
	}

	return true, nil
}

// isEclipseCheRelatedObj indicates if there is a object with
// the label 'app.kubernetes.io/part-of=che.eclipse.org' in a che namespace
func isEclipseCheRelatedObj(mgr controller.Manager, obj handler.MapObject) (bool, controller.Request) {
	checlusters := &orgv1.CheClusterList{}
	if err := mgr.GetClient().List(context.TODO(), checlusters, &client.ListOptions{}); err != nil {
		return false, controller.Request{}
	}

	if len(checlusters.Items) != 1 {
		return false, controller.Request{}
	}

	if value, exists := obj.Meta.GetLabels()[deploy.KubernetesPartOfLabelKey]; !exists || value != deploy.CheEclipseOrg {
		// Labels do not match
		return false, controller.Request{}
	}

	return true, controller.Request{
		NamespacedName: types.NamespacedName{
			Namespace: checlusters.Items[0].Namespace,
			Name:      checlusters.Items[0].Name,
		},
	}
}

func (r *CheClusterReconciler) reconcileFinalizers(deployContext *deploy.DeployContext) {
	if util.IsOpenShift && util.IsOAuthEnabled(deployContext.CheCluster) {
		if err := deploy.ReconcileOAuthClientFinalizer(deployContext); err != nil {
			logrus.Error(err)
		}
	}

	if util.IsOpenShift4 && util.IsInitialOpenShiftOAuthUserEnabled(deployContext.CheCluster) {
		if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			r.userHandler.DeleteOAuthInitialUser(deployContext)
		}
	}

	if util.IsNativeUserModeEnabled(deployContext.CheCluster) {
		if _, err := reconcileGatewayPermissionsFinalizers(deployContext); err != nil {
			logrus.Error(err)
		}
	}

	if _, err := reconcileWorkspacePermissionsFinalizers(deployContext); err != nil {
		logrus.Error(err)
	}

	if err := deploy.ReconcileConsoleLinkFinalizer(deployContext); err != nil {
		logrus.Error(err)
	}

	if len(deployContext.CheCluster.Spec.Server.CheClusterRoles) > 0 {
		cheClusterRoles := strings.Split(deployContext.CheCluster.Spec.Server.CheClusterRoles, ",")
		for _, cheClusterRole := range cheClusterRoles {
			cheClusterRole := strings.TrimSpace(cheClusterRole)
			cheClusterRoleBindingName := cheClusterRole
			if err := deploy.ReconcileClusterRoleBindingFinalizer(deployContext, cheClusterRoleBindingName); err != nil {
				logrus.Error(err)
			}

			// Removes any legacy CRB https://github.com/eclipse/che/issues/19506
			cheClusterRoleBindingName = deploy.GetLegacyUniqueClusterRoleBindingName(deployContext, CheServiceAccountName, cheClusterRole)
			if err := deploy.ReconcileLegacyClusterRoleBindingFinalizer(deployContext, cheClusterRoleBindingName); err != nil {
				logrus.Error(err)
			}
		}
	}
}

func (r *CheClusterReconciler) GetCR(request controller.Request) (instance *orgv1.CheCluster, err error) {
	instance = &orgv1.CheCluster{}
	err = r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		r.Log.Error(err, "Failed to get %s CR: %s", "Cluster name", instance.Name)
		return nil, err
	}
	return instance, nil
}

// ValidateCheCR checks Che CR configuration.
// It should detect:
// - configurations which miss required field(s) to deploy Che
// - self-contradictory configurations
// - configurations with which it is impossible to deploy Che
func validateCheCR(deployContext *deploy.DeployContext) (bool, error) {
	if !util.IsTestMode() {
		if !util.IsOpenShift {
			if deployContext.CheCluster.Spec.K8s.IngressDomain == "" {
				return false, fmt.Errorf("Required field \"spec.K8s.IngressDomain\" is not set")
			}
		}

		workspaceNamespaceDefault := util.GetWorkspaceNamespaceDefault(deployContext.CheCluster)
		if strings.Index(workspaceNamespaceDefault, "<username>") == -1 && strings.Index(workspaceNamespaceDefault, "<userid>") == -1 {
			return false, fmt.Errorf(`Namespace strategies other than 'per user' is not supported anymore. Using the <username> or <userid> placeholder is required in the 'spec.server.workspaceNamespaceDefault' field. The current value is: %s`, workspaceNamespaceDefault)
		}
	}

	return true, nil
}

// Read proxy configuration
func readProxyConfiguration(deployContext *deploy.DeployContext) (bool, error) {
	proxy, err := getProxyConfiguration(deployContext)
	if err != nil {
		return false, err
	}

	// Assign Proxy to the deploy context
	deployContext.Proxy = proxy
	return true, nil
}

func syncOpenShiftCertificates(deployContext *deploy.DeployContext) (bool, error) {
	if deployContext.Proxy.TrustedCAMapName != "" {
		return putOpenShiftCertsIntoConfigMap(deployContext)
	}

	return true, nil
}

func syncImagePuller(deployContext *deploy.DeployContext) (bool, error) {
	result, err := deploy.ReconcileImagePuller(deployContext)
	return !result.Requeue && result.RequeueAfter == 0 && err == nil, err
}

func syncSelfSignedCertificate(deployContext *deploy.DeployContext) (bool, error) {
	// Detect whether self-signed certificate is used
	selfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		logrus.Error(err, "Failed to detect if self-signed certificate used.")
		return false, err
	}

	if util.IsOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if selfSignedCertUsed ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(util.IsOpenShift4 && util.IsOAuthEnabled(deployContext.CheCluster) && !deployContext.CheCluster.Spec.Server.TlsSupport) {
			if err := deploy.CreateTLSSecretFromEndpoint(deployContext, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
				return false, err
			}
		}

		if util.IsOAuthEnabled(deployContext.CheCluster) {
			// create a secret with OpenShift API crt to be added to keystore that RH SSO will consume
			apiUrl, apiInternalUrl, err := util.GetOpenShiftAPIUrls()
			if err != nil {
				logrus.Errorf("Failed to get OpenShift cluster public hostname. A secret with API crt will not be created and consumed by RH-SSO/Keycloak")
			} else {
				baseURL := map[bool]string{true: apiInternalUrl, false: apiUrl}[apiInternalUrl != ""]
				if err := deploy.CreateTLSSecretFromEndpoint(deployContext, baseURL, "openshift-api-crt"); err != nil {
					return false, err
				}
			}
		}
	} else {
		// Handle Che TLS certificates on Kubernetes infrastructure
		if deployContext.CheCluster.Spec.Server.TlsSupport {
			if deployContext.CheCluster.Spec.K8s.TlsSecretName != "" {
				// Self-signed certificate should be created to secure Che ingresses
				reconcile, err := deploy.K8sHandleCheTLSSecrets(deployContext)
				if reconcile.Requeue || reconcile.RequeueAfter > 0 || err != nil {
					return false, err
				}
			} else if selfSignedCertUsed {
				// Use default self-signed ingress certificate
				if err := deploy.CreateTLSSecretFromEndpoint(deployContext, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

// Create service account "che" for che-server component.
// "che" is the one which token is used to create workspace objects.
// Notice: Also we have on more "che-workspace" SA used by plugins like exec, terminal, metrics with limited privileges.
func syncServiceAccount(deployContext *deploy.DeployContext) (bool, error) {
	return deploy.SyncServiceAccountToCluster(deployContext, CheServiceAccountName)
}

func syncCheClusterRoles(deployContext *deploy.DeployContext) (bool, error) {
	if len(deployContext.CheCluster.Spec.Server.CheClusterRoles) > 0 {
		cheClusterRoles := strings.Split(deployContext.CheCluster.Spec.Server.CheClusterRoles, ",")
		for _, cheClusterRole := range cheClusterRoles {
			cheClusterRole := strings.TrimSpace(cheClusterRole)
			cheClusterRoleBindingName := cheClusterRole

			done, err := deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(deployContext, cheClusterRoleBindingName, CheServiceAccountName, cheClusterRole)
			if !done {
				return false, err
			}
		}
	}

	return true, nil
}

// If the user specified an additional cluster role to use for the Che workspace, create a role binding for it
// Use a role binding instead of a cluster role binding to keep the additional access scoped to the workspace's namespace
func syncWorkspaceClusterRoles(deployContext *deploy.DeployContext) (bool, error) {
	workspaceClusterRole := deployContext.CheCluster.Spec.Server.CheWorkspaceClusterRole
	if workspaceClusterRole != "" {
		return deploy.SyncRoleBindingToCluster(deployContext, "che-workspace-custom", "view", workspaceClusterRole, "ClusterRole")
	}

	return true, nil
}

func syncDefaults(deployContext *deploy.DeployContext) (bool, error) {
	err := GenerateAndSaveFields(deployContext)
	return err == nil, err
}

func syncPostgres(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.Spec.Database.ExternalDb {
		postgres := postgres.NewPostgres(deployContext)
		return postgres.SyncAll()
	}

	return true, nil
}

// we have to expose che endpoint independently of syncing other server
// resources since che host is used for dashboard deployment and che config map
func syncServerEndpoint(deployContext *deploy.DeployContext) (bool, error) {
	server := server.NewServer(deployContext)
	return server.ExposeCheServiceAndEndpoint()
}

// create and provision Keycloak related objects
func syncIdentityProvider(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider {
		return identity_provider.SyncIdentityProviderToCluster(deployContext)
	} else {
		keycloakURL := deployContext.CheCluster.Spec.Auth.IdentityProviderURL
		if deployContext.CheCluster.Status.KeycloakURL != keycloakURL {
			deployContext.CheCluster.Status.KeycloakURL = keycloakURL
			if err := deploy.UpdateCheCRStatus(deployContext, "status: Keycloak URL", keycloakURL); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func syncPluginRegistry(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		pluginRegistry := pluginregistry.NewPluginRegistry(deployContext)
		return pluginRegistry.SyncAll()
	} else {
		if deployContext.CheCluster.Spec.Server.PluginRegistryUrl != deployContext.CheCluster.Status.PluginRegistryURL {
			deployContext.CheCluster.Status.PluginRegistryURL = deployContext.CheCluster.Spec.Server.PluginRegistryUrl
			if err := deploy.UpdateCheCRStatus(deployContext, "status: Plugin Registry URL", deployContext.CheCluster.Spec.Server.PluginRegistryUrl); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func syncDevfileRegistry(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.Spec.Server.ExternalDevfileRegistry {
		devfileRegistry := devfileregistry.NewDevfileRegistry(deployContext)
		return devfileRegistry.SyncAll()
	}
	return true, nil
}

func syncDashboard(deployContext *deploy.DeployContext) (bool, error) {
	d := dashboard.NewDashboard(deployContext)
	return d.SyncAll()
}

func syncGateway(deployContext *deploy.DeployContext) (bool, error) {
	err := gateway.SyncGatewayToCluster(deployContext)
	return err == nil, nil
}

func syncServer(deployContext *deploy.DeployContext) (bool, error) {
	server := server.NewServer(deployContext)
	return server.SyncAll()
}

// Delete OpenShift identity provider if OpenShift oAuth is false in spec
// but OpenShiftoAuthProvisioned is true in CR status, e.g. when oAuth has been turned on and then turned off
func reconcileIdentityProvider(deployContext *deploy.DeployContext) (bool, error) {
	deleted, _ := identity_provider.ReconcileIdentityProvider(deployContext)
	if deleted {
		// ignore error
		deploy.DeleteFinalizer(deployContext, deploy.OAuthFinalizerName)
		for {
			deployContext.CheCluster.Status.OpenShiftoAuthProvisioned = false
			if err := deploy.UpdateCheCRStatus(deployContext, "status: provisioned with OpenShift identity provider", "false"); err != nil &&
				errors.IsConflict(err) {
				deploy.ReloadCheClusterCR(deployContext)
				continue
			}
			break
		}
		for {
			deployContext.CheCluster.Spec.Auth.OAuthSecret = ""
			deployContext.CheCluster.Spec.Auth.OAuthClientName = ""

			if err := deploy.UpdateCheCRStatus(deployContext, "clean oAuth secret name and client name", ""); err != nil &&
				errors.IsConflict(err) {
				deploy.ReloadCheClusterCR(deployContext)
				continue
			}
			break
		}
	}

	return true, nil
}

func deleteOAuthInitialUserIfNeeded(deployContext *deploy.DeployContext) (bool, error) {
	if util.IsOpenShift4 && util.IsDeleteOAuthInitialUser(deployContext.CheCluster) {
		userHandler := NewOpenShiftOAuthUserHandler(deployContext.ClusterAPI.NonCachedClient)
		if err := userHandler.DeleteOAuthInitialUser(deployContext); err != nil {
			logrus.Errorf("Unable to delete initial OpenShift OAuth user from a cluster. Cause: %s", err.Error())

			// don't try one more time
			deployContext.CheCluster.Spec.Auth.InitialOpenShiftOAuthUser = nil
			err := deploy.UpdateCheCRSpec(deployContext, "initialOpenShiftOAuthUser", "nil")
			return false, err
		}

		deployContext.CheCluster.Spec.Auth.OpenShiftoAuth = nil
		deployContext.CheCluster.Spec.Auth.InitialOpenShiftOAuthUser = nil
		updateFields := map[string]string{
			"openShiftoAuth":            "nil",
			"initialOpenShiftOAuthUser": "nil",
		}

		if err := deploy.UpdateCheCRSpecByFields(deployContext, updateFields); err != nil {
			return false, err
		}

		return true, nil
	}

	// Update status if OpenShift initial user is deleted (in the previous step)
	if deployContext.CheCluster.Spec.Auth.InitialOpenShiftOAuthUser == nil && deployContext.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret != "" {
		secret := &corev1.Secret{}
		exists, err := getOpenShiftOAuthUserCredentialsSecret(deployContext, secret)
		if err != nil {
			// We should `Requeue` since we deal with cluster scope objects
			return false, err
		} else if !exists {
			deployContext.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret = ""
			if err := deploy.UpdateCheCRStatus(deployContext, "openShiftOAuthUserCredentialsSecret", ""); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}
