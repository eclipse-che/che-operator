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
	"reflect"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	userv1 "github.com/openshift/api/user/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

var (
	// CheServiceAccountName - service account name for che-server.
	CheServiceAccountName = "che"
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
	Scheme *runtime.Scheme

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
	// the namespace to which to limit the reconciliation. If empty, all namespaces are considered
	namespace string
}

// NewReconciler returns a new CheClusterReconciler
func NewReconciler(mgr ctrl.Manager, namespace string, discoveryClient *discovery.DiscoveryClient) (*CheClusterReconciler, error) {
	noncachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return nil, err
	}
	return &CheClusterReconciler{
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("CheCluster"),

		client:            mgr.GetClient(),
		nonCachedClient:   noncachedClient,
		discoveryClient:   discoveryClient,
		userHandler:       NewOpenShiftOAuthUserHandler(noncachedClient),
		permissionChecker: &K8sApiPermissionChecker{},
		namespace:         namespace,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	var toTrustedBundleConfigMapRequestMapper handler.MapFunc = func(obj client.Object) []ctrl.Request {
		isTrusted, reconcileRequest := IsTrustedBundleConfigMap(r.nonCachedClient, r.namespace, obj)
		if isTrusted {
			return []ctrl.Request{reconcileRequest}
		}
		return []ctrl.Request{}
	}

	var toEclipseCheRelatedObjRequestMapper handler.MapFunc = func(obj client.Object) []ctrl.Request {
		isEclipseCheRelatedObj, reconcileRequest := IsEclipseCheRelatedObj(r.nonCachedClient, r.namespace, obj)
		if isEclipseCheRelatedObj {
			return []ctrl.Request{reconcileRequest}
		}
		return []ctrl.Request{}
	}

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
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
			handler.EnqueueRequestsFromMapFunc(toTrustedBundleConfigMapRequestMapper),
			builder.WithPredicates(onAllExceptGenericEventsPredicate),
		).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(toEclipseCheRelatedObjRequestMapper),
			builder.WithPredicates(onAllExceptGenericEventsPredicate),
		).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(toEclipseCheRelatedObjRequestMapper),
			builder.WithPredicates(onAllExceptGenericEventsPredicate),
		)

	if isOpenShift {
		controllerBuilder = controllerBuilder.Watches(&source.Kind{Type: &routev1.Route{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		})
	} else {
		controllerBuilder = controllerBuilder.Watches(&source.Kind{Type: &networking.Ingress{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		})
	}

	if r.namespace != "" {
		controllerBuilder = controllerBuilder.WithEventFilter(util.InNamespaceEventFilter(r.namespace))
	}

	return controllerBuilder.
		For(&orgv1.CheCluster{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.5/pkg/reconcile
func (r *CheClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("checluster", req.NamespacedName)

	clusterAPI := deploy.ClusterAPI{
		Client:          r.client,
		NonCachedClient: r.nonCachedClient,
		DiscoveryClient: r.discoveryClient,
		Scheme:          r.Scheme,
	}
	// Fetch the CheCluster instance
	tests := r.tests
	instance, err := r.GetCR(req)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	deployContext := &deploy.DeployContext{
		ClusterAPI: clusterAPI,
		CheCluster: instance,
	}

	if isCheGoingToBeUpdated(instance) {
		// Current operator is newer than deployed Che
		backupCR, err := getBackupCRForUpdate(deployContext)
		if err != nil {
			if errors.IsNotFound(err) {
				// Create a backup before updating current installation
				if err := requestBackup(deployContext); err != nil {
					return ctrl.Result{}, err
				}
				// Backup request is successfully submitted
				// Give some time for the backup
				return ctrl.Result{RequeueAfter: time.Second * 15}, nil
			}
			return ctrl.Result{}, err
		}
		if backupCR.Status.State == orgv1.STATE_IN_PROGRESS || backupCR.Status.State == "" {
			// Backup is still in progress
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		// Backup is done or failed
		// Proceed anyway
	}

	// Reconcile finalizers before CR is deleted
	r.reconcileFinalizers(deployContext)

	// Reconcile the imagePuller section of the CheCluster
	imagePullerResult, err := deploy.ReconcileImagePuller(deployContext)
	if err != nil {
		return imagePullerResult, err
	}
	if imagePullerResult.Requeue || imagePullerResult.RequeueAfter > 0 {
		return imagePullerResult, err
	}

	// Check Che CR correctness
	if !util.IsTestMode() {
		if err := ValidateCheCR(instance); err != nil {
			// Che cannot be deployed with current configuration.
			// Print error message in logs and wait until the configuration is changed.
			logrus.Error(err)
			if err := deploy.SetStatusDetails(deployContext, failedValidationReason, err.Error(), ""); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
	}

	if util.IsOpenShift4 && util.IsDeleteOAuthInitialUser(instance) {
		if err := r.userHandler.DeleteOAuthInitialUser(deployContext); err != nil {
			logrus.Errorf("Unable to delete initial OpenShift OAuth user from a cluster. Cause: %s", err.Error())
			instance.Spec.Auth.InitialOpenShiftOAuthUser = nil
			err := deploy.UpdateCheCRSpec(deployContext, "initialOpenShiftOAuthUser", "nil")
			return reconcile.Result{}, err
		}

		instance.Spec.Auth.OpenShiftoAuth = nil
		instance.Spec.Auth.InitialOpenShiftOAuthUser = nil
		updateFields := map[string]string{
			"openShiftoAuth":            "nil",
			"initialOpenShiftOAuthUser": "nil",
		}

		if err := deploy.UpdateCheCRSpecByFields(deployContext, updateFields); err != nil {
			return reconcile.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Update status if OpenShift initial user is deleted (in the previous step)
	if instance.Spec.Auth.InitialOpenShiftOAuthUser == nil && instance.Status.OpenShiftOAuthUserCredentialsSecret != "" {
		secret := &corev1.Secret{}
		exists, err := getOpenShiftOAuthUserCredentialsSecret(deployContext, secret)
		if err != nil {
			// We should `Requeue` since we deal with cluster scope objects
			return ctrl.Result{RequeueAfter: time.Second}, err
		} else if !exists {
			instance.Status.OpenShiftOAuthUserCredentialsSecret = ""
			if err := deploy.UpdateCheCRStatus(deployContext, "openShiftOAuthUserCredentialsSecret", ""); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if util.IsOpenShift && instance.Spec.DevWorkspace.Enable && instance.Spec.Auth.NativeUserMode == nil {
		newNativeUserModeValue := util.NewBoolPointer(true)
		instance.Spec.Auth.NativeUserMode = newNativeUserModeValue
		if err := deploy.UpdateCheCRSpec(deployContext, "nativeUserMode", strconv.FormatBool(*newNativeUserModeValue)); err != nil {
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	if util.IsOpenShift && instance.Spec.Auth.OpenShiftoAuth == nil {
		if reconcileResult, err := r.autoEnableOAuth(deployContext, req, util.IsOpenShift4); err != nil {
			return reconcileResult, err
		}
	}

	// Reconcile Dev Workspace Operator
	done, err := devworkspace.ReconcileDevWorkspace(deployContext)
	if !done {
		if err != nil {
			r.Log.Error(err, "")
		}
		// We should `Requeue` since we don't watch Dev Workspace controller objects
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

	// Read proxy configuration
	proxy, err := GetProxyConfiguration(deployContext)
	if err != nil {
		r.Log.Error(err, "Error on reading proxy configuration")
		return ctrl.Result{}, err
	}
	// Assign Proxy to the deploy context
	deployContext.Proxy = proxy

	if proxy.TrustedCAMapName != "" {
		provisioned, err := r.putOpenShiftCertsIntoConfigMap(deployContext)
		if !provisioned {
			configMapName := instance.Spec.Server.ServerTrustStoreConfigMapName
			if err != nil {
				r.Log.Error(err, "Error on provisioning", "config map", configMapName)
			} else {
				r.Log.Error(err, "Waiting on provisioning", "config map", configMapName)
			}
			return ctrl.Result{}, err
		}
	}

	// Detect whether self-signed certificate is used
	selfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		r.Log.Error(err, "Failed to detect if self-signed certificate used.")
		return ctrl.Result{}, err
	}

	if util.IsOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if selfSignedCertUsed ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(util.IsOpenShift4 && util.IsOAuthEnabled(instance) && !instance.Spec.Server.TlsSupport) {
			if err := deploy.CreateTLSSecretFromEndpoint(deployContext, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
				return ctrl.Result{}, err
			}
		}

		if util.IsOAuthEnabled(instance) {
			// create a secret with OpenShift API crt to be added to keystore that RH SSO will consume
			apiUrl, apiInternalUrl, err := util.GetOpenShiftAPIUrls()
			if err != nil {
				logrus.Errorf("Failed to get OpenShift cluster public hostname. A secret with API crt will not be created and consumed by RH-SSO/Keycloak")
			} else {
				baseURL := map[bool]string{true: apiInternalUrl, false: apiUrl}[apiInternalUrl != ""]
				if err := deploy.CreateTLSSecretFromEndpoint(deployContext, baseURL, "openshift-api-crt"); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	} else {
		// Handle Che TLS certificates on Kubernetes infrastructure
		if instance.Spec.Server.TlsSupport {
			if instance.Spec.K8s.TlsSecretName != "" {
				// Self-signed certificate should be created to secure Che ingresses
				result, err := deploy.K8sHandleCheTLSSecrets(deployContext)
				if result.Requeue || result.RequeueAfter > 0 {
					if err != nil {
						logrus.Error(err)
					}
					if !tests {
						return result, err
					}
				}
			} else if selfSignedCertUsed {
				// Use default self-signed ingress certificate
				if err := deploy.CreateTLSSecretFromEndpoint(deployContext, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	// Make sure that CA certificates from all marked config maps are merged into single config map to be propageted to Che components
	done, err = deploy.SyncAdditionalCACertsConfigMapToCluster(deployContext)
	if err != nil {
		r.Log.Error(err, "Error updating additional CA config map")
		return ctrl.Result{}, err
	}
	if !done && !tests {
		// Config map update is in progress
		// Return and do not force reconcile. When update finishes it will trigger reconcile loop.
		return ctrl.Result{}, err
	}

	if err := deploy.SetStatusDetails(deployContext, "", "", ""); err != nil {
		return ctrl.Result{}, err
	}

	// Create service account "che" for che-server component.
	// "che" is the one which token is used to create workspace objects.
	// Notice: Also we have on more "che-workspace" SA used by plugins like exec, terminal, metrics with limited privileges.
	done, err = deploy.SyncServiceAccountToCluster(deployContext, CheServiceAccountName)
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

	if done, err = r.reconcileGatewayPermissions(deployContext); !done {
		if err != nil {
			logrus.Error(err)
		}
		// reconcile after 1 seconds since we deal with cluster objects
		return reconcile.Result{RequeueAfter: time.Second}, err
	}

	done, err = r.reconcileWorkspacePermissions(deployContext)
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		// reconcile after 1 seconds since we deal with cluster objects
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

	if len(instance.Spec.Server.CheClusterRoles) > 0 {
		cheClusterRoles := strings.Split(instance.Spec.Server.CheClusterRoles, ",")
		for _, cheClusterRole := range cheClusterRoles {
			cheClusterRole := strings.TrimSpace(cheClusterRole)
			cheClusterRoleBindingName := cheClusterRole
			done, err := deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(deployContext, cheClusterRoleBindingName, CheServiceAccountName, cheClusterRole)
			if !tests {
				if !done {
					logrus.Infof("Waiting on cluster role binding '%s' to be created", cheClusterRoleBindingName)
					if err != nil {
						logrus.Error(err)
					}
					return ctrl.Result{RequeueAfter: time.Second}, err
				}
			}
		}
	}

	// If the user specified an additional cluster role to use for the Che workspace, create a role binding for it
	// Use a role binding instead of a cluster role binding to keep the additional access scoped to the workspace's namespace
	workspaceClusterRole := instance.Spec.Server.CheWorkspaceClusterRole
	if workspaceClusterRole != "" {
		done, err := deploy.SyncRoleBindingToCluster(deployContext, "che-workspace-custom", "view", workspaceClusterRole, "ClusterRole")
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	if err := r.GenerateAndSaveFields(deployContext, req); err != nil {
		instance, _ = r.GetCR(req)
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
	}

	if !deployContext.CheCluster.Spec.Database.ExternalDb {
		postgres := postgres.NewPostgres(deployContext)
		done, err = postgres.SyncAll()
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return ctrl.Result{}, err
		}
	}

	// we have to expose che endpoint independently of syncing other server
	// resources since che host is used for dashboard deployment and che config map
	server := server.NewServer(deployContext)
	done, err = server.ExposeCheServiceAndEndpoint()
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		return ctrl.Result{}, err
	}

	// create and provision Keycloak related objects
	if !instance.Spec.Auth.ExternalIdentityProvider {
		provisioned, err := identity_provider.SyncIdentityProviderToCluster(deployContext)
		if !provisioned {
			if err != nil {
				logrus.Errorf("Error provisioning the identity provider to cluster: %v", err)
			}
			return ctrl.Result{}, err
		}
	} else {
		keycloakURL := instance.Spec.Auth.IdentityProviderURL
		if instance.Status.KeycloakURL != keycloakURL {
			instance.Status.KeycloakURL = keycloakURL
			if err := deploy.UpdateCheCRStatus(deployContext, "status: Keycloak URL", keycloakURL); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	devfileRegistry := devfileregistry.NewDevfileRegistry(deployContext)
	if !instance.Spec.Server.ExternalDevfileRegistry {
		done, err := devfileRegistry.SyncAll()
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return ctrl.Result{}, err
		}
	}

	if !instance.Spec.Server.ExternalPluginRegistry {
		pluginRegistry := pluginregistry.NewPluginRegistry(deployContext)
		done, err := pluginRegistry.SyncAll()
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return ctrl.Result{}, err
		}
	} else {
		if instance.Spec.Server.PluginRegistryUrl != instance.Status.PluginRegistryURL {
			instance.Status.PluginRegistryURL = instance.Spec.Server.PluginRegistryUrl
			if err := deploy.UpdateCheCRStatus(deployContext, "status: Plugin Registry URL", instance.Spec.Server.PluginRegistryUrl); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	d := dashboard.NewDashboard(deployContext)
	done, err = d.Reconcile()
	if !done {
		if err != nil {
			logrus.Errorf("Error provisioning '%s' to cluster: %v", d.GetComponentName(), err)
		}
		return ctrl.Result{}, err
	}

	err = gateway.SyncGatewayToCluster(deployContext)
	if err != nil {
		logrus.Errorf("Failed to create the Server Gateway: %s", err)
		return ctrl.Result{}, err
	}

	done, err = server.SyncAll()
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		return reconcile.Result{}, err
	}

	// we can now try to create consolelink, after che instance is available
	done, err = deploy.ReconcileConsoleLink(deployContext)
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		// We should `Requeue` since we created cluster object
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

	// Delete OpenShift identity provider if OpenShift oAuth is false in spec
	// but OpenShiftoAuthProvisioned is true in CR status, e.g. when oAuth has been turned on and then turned off
	deleted, err := identity_provider.ReconcileIdentityProvider(deployContext)
	if deleted {
		// ignore error
		deploy.DeleteFinalizer(deployContext, deploy.OAuthFinalizerName)
		for {
			instance.Status.OpenShiftoAuthProvisioned = false
			if err := deploy.UpdateCheCRStatus(deployContext, "status: provisioned with OpenShift identity provider", "false"); err != nil &&
				errors.IsConflict(err) {
				instance, _ = r.GetCR(req)
				continue
			}
			break
		}
		for {
			instance.Spec.Auth.OAuthSecret = ""
			instance.Spec.Auth.OAuthClientName = ""
			if err := deploy.UpdateCheCRStatus(deployContext, "clean oAuth secret name and client name", ""); err != nil &&
				errors.IsConflict(err) {
				instance, _ = r.GetCR(req)
				continue
			}
			break
		}
	}

	return ctrl.Result{}, nil
}

func (r *CheClusterReconciler) autoEnableOAuth(deployContext *deploy.DeployContext, request ctrl.Request, isOpenShift4 bool) (reconcile.Result, error) {
	var message, reason string
	oauth := false
	cr := deployContext.CheCluster
	if isOpenShift4 {
		openshitOAuth, err := GetOpenshiftOAuth(deployContext.ClusterAPI.NonCachedClient)
		if err != nil {
			message = "Unable to get Openshift oAuth. Cause: " + err.Error()
			logrus.Error(message)
			reason = failedUnableToGetOAuth
		} else {
			if len(openshitOAuth.Spec.IdentityProviders) > 0 {
				oauth = true
			} else if util.IsNativeUserModeEnabled(deployContext.CheCluster) {
				// enable OpenShift OAuth without adding initial OpenShift OAuth user
				// since kubeadmin is a valid user for native user mode
				oauth = true
			} else if util.IsInitialOpenShiftOAuthUserEnabled(cr) {
				provisioned, err := r.userHandler.SyncOAuthInitialUser(openshitOAuth, deployContext)
				if err != nil {
					message = warningNoIdentityProvidersMessage + " Operator tried to create initial OpenShift OAuth user for HTPasswd identity provider, but failed. Cause: " + err.Error()
					logrus.Error(message)
					logrus.Info("To enable OpenShift OAuth, please add identity provider first: " + howToAddIdentityProviderLinkOS4)
					reason = failedNoIdentityProviders
					// Don't try to create initial user any more, che-operator shouldn't hang on this step.
					cr.Spec.Auth.InitialOpenShiftOAuthUser = nil
					if err := deploy.UpdateCheCRStatus(deployContext, "initialOpenShiftOAuthUser", ""); err != nil {
						return reconcile.Result{}, err
					}
					oauth = false
				} else {
					if !provisioned {
						return reconcile.Result{}, err
					}
					oauth = true
					if deployContext.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret == "" {
						deployContext.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret = openShiftOAuthUserCredentialsSecret
						if err := deploy.UpdateCheCRStatus(deployContext, "openShiftOAuthUserCredentialsSecret", openShiftOAuthUserCredentialsSecret); err != nil {
							return reconcile.Result{}, err
						}
					}
				}
			}
		}
	} else { // Openshift 3
		users := &userv1.UserList{}
		listOptions := &client.ListOptions{}
		if err := r.nonCachedClient.List(context.TODO(), users, listOptions); err != nil {
			message = failedUnableToGetOpenshiftUsers + " Cause: " + err.Error()
			logrus.Error(message)
			reason = failedNoOpenshiftUser
		} else {
			oauth = len(users.Items) >= 1
			if !oauth {
				message = warningNoRealUsersMessage + " " + howToConfigureOAuthLinkOS3
				logrus.Warn(message)
				reason = failedNoOpenshiftUser
			}
		}
	}

	newOAuthValue := util.NewBoolPointer(oauth)
	if !reflect.DeepEqual(newOAuthValue, cr.Spec.Auth.OpenShiftoAuth) {
		cr.Spec.Auth.OpenShiftoAuth = newOAuthValue
		if err := deploy.UpdateCheCRSpec(deployContext, "openShiftoAuth", strconv.FormatBool(oauth)); err != nil {
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	if message != "" && reason != "" {
		if err := deploy.SetStatusDetails(deployContext, message, reason, ""); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
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
		if _, err := r.reconcileGatewayPermissionsFinalizers(deployContext); err != nil {
			logrus.Error(err)
		}
	}

	if _, err := r.reconcileWorkspacePermissionsFinalizers(deployContext); err != nil {
		logrus.Error(err)
	}

	if err := deploy.ReconcileConsoleLinkFinalizer(deployContext); err != nil {
		logrus.Error(err)
	}

	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		done, err := dashboard.NewDashboard(deployContext).Finalize()
		if !done {
			logrus.Error(err)
		}
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

func (r *CheClusterReconciler) GetCR(request ctrl.Request) (instance *orgv1.CheCluster, err error) {
	instance = &orgv1.CheCluster{}
	err = r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		r.Log.Error(err, "Failed to get %s CR: %s", "Cluster name", instance.Name)
		return nil, err
	}
	return instance, nil
}
