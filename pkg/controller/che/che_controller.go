//
// Copyright (c) 2012-2019 Red Hat, Inc.
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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/dashboard"
	devworkspace "github.com/eclipse-che/che-operator/pkg/deploy/dev-workspace"
	"github.com/eclipse-che/che-operator/pkg/deploy/devfileregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/deploy/pluginregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/postgres"
	"github.com/eclipse-che/che-operator/pkg/deploy/server"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_che")

// Add creates a new CheCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	reconciler, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, reconciler)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	noncachedClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	return &ReconcileChe{
		client:            mgr.GetClient(),
		nonCachedClient:   noncachedClient,
		scheme:            mgr.GetScheme(),
		discoveryClient:   discoveryClient,
		userHandler:       NewOpenShiftOAuthUserHandler(noncachedClient),
		permissionChecker: &K8sApiPermissionChecker{},
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
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

	// Create a new controller
	c, err := controller.New("che-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// register OpenShift specific types in the scheme
	if util.IsOpenShift {
		if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift route to scheme: %s", err)
		}
		if err := oauth.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift OAuth to scheme: %s", err)
		}
		if err := userv1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift User to scheme: %s", err)
		}
		if err := oauthv1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift OAuth to scheme: %s", err)
		}
		if err := configv1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift Config to scheme: %s", err)
		}
		if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift Core to scheme: %s", err)
		}
		if err := consolev1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift ConsoleLink to scheme: %s", err)
		}
	}

	// register Extension in the scheme
	if err := apiextensionsv1.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Errorf("Failed to add Extension to scheme: %s", err)
	}

	// register Admission in the scheme
	if err := admissionregistrationv1.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Errorf("Failed to add Admission to scheme: %s", err)
	}

	// register RBAC in the scheme
	if err := rbac.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Errorf("Failed to add RBAC to scheme: %s", err)
	}

	// Watch for changes to primary resource CheCluster
	err = c.Watch(&source.Kind{Type: &orgv1.CheCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner CheCluster

	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	}); err != nil {
		return err
	}

	if err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	}); err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	})
	if err != nil {
		return err
	}

	var toTrustedBundleConfigMapRequestMapper handler.ToRequestsFunc = func(obj handler.MapObject) []reconcile.Request {
		isTrusted, reconcileRequest := isTrustedBundleConfigMap(mgr, obj)
		if isTrusted {
			return []reconcile.Request{reconcileRequest}
		}
		return []reconcile.Request{}
	}
	if err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: toTrustedBundleConfigMapRequestMapper,
	}, onAllExceptGenericEventsPredicate); err != nil {
		return err
	}

	var toEclipseCheSecretRequestMapper handler.ToRequestsFunc = func(obj handler.MapObject) []reconcile.Request {
		isEclipseCheSecret, reconcileRequest := isEclipseCheSecret(mgr, obj)
		if isEclipseCheSecret {
			return []reconcile.Request{reconcileRequest}
		}
		return []reconcile.Request{}
	}
	if err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: toEclipseCheSecretRequestMapper,
	}, onAllExceptGenericEventsPredicate); err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &rbac.Role{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &rbac.RoleBinding{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	})
	if err != nil {
		return err
	}

	if util.IsOpenShift {
		err = c.Watch(&source.Kind{Type: &routev1.Route{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		})
		if err != nil {
			return err
		}
	} else {
		err = c.Watch(&source.Kind{Type: &v1beta1.Ingress{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		})
		if err != nil {
			return err
		}
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheCluster{},
	})
	if err != nil {
		return err
	}
	return nil
}

var (
	_ reconcile.Reconciler = &ReconcileChe{}

	// CheServiceAccountName - service account name for che-server.
	CheServiceAccountName = "che"
)

// ReconcileChe reconciles a CheCluster object
type ReconcileChe struct {
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
	scheme            *runtime.Scheme
	tests             bool
	userHandler       OpenShiftOAuthUserHandler
	permissionChecker PermissionChecker
}

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

// Reconcile reads that state of the cluster for a CheCluster object and makes changes based on the state read
// and what is in the CheCluster.Spec. The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileChe) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	clusterAPI := deploy.ClusterAPI{
		Client:          r.client,
		NonCachedClient: r.nonCachedClient,
		DiscoveryClient: r.discoveryClient,
		Scheme:          r.scheme,
	}
	// Fetch the CheCluster instance
	tests := r.tests
	instance, err := r.GetCR(request)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	deployContext := &deploy.DeployContext{
		ClusterAPI: clusterAPI,
		CheCluster: instance,
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

		return reconcile.Result{}, nil
	}

	// Update status if OpenShift initial user is deleted (in the previous step)
	if instance.Spec.Auth.InitialOpenShiftOAuthUser == nil && instance.Status.OpenShiftOAuthUserCredentialsSecret != "" {
		secret := &corev1.Secret{}
		exists, err := getOpenShiftOAuthUserCredentialsSecret(deployContext, secret)
		if err != nil {
			// We should `Requeue` since we deal with cluster scope objects
			return reconcile.Result{RequeueAfter: time.Second}, err
		} else if !exists {
			instance.Status.OpenShiftOAuthUserCredentialsSecret = ""
			if err := deploy.UpdateCheCRStatus(deployContext, "openShiftOAuthUserCredentialsSecret", ""); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if util.IsOpenShift && instance.Spec.Auth.OpenShiftoAuth == nil {
		if reconcileResult, err := r.autoEnableOAuth(deployContext, request, util.IsOpenShift4); err != nil {
			return reconcileResult, err
		}
	}

	// Reconcile Dev Workspace Operator
	done, err := devworkspace.ReconcileDevWorkspace(deployContext)
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		// We should `Requeue` since we don't watch Dev Workspace controller objects
		return reconcile.Result{RequeueAfter: time.Second}, err
	}

	// Read proxy configuration
	proxy, err := r.getProxyConfiguration(deployContext)
	if err != nil {
		logrus.Errorf("Error on reading proxy configuration: %v", err)
		return reconcile.Result{}, err
	}
	// Assign Proxy to the deploy context
	deployContext.Proxy = proxy

	if proxy.TrustedCAMapName != "" {
		provisioned, err := r.putOpenShiftCertsIntoConfigMap(deployContext)
		if !provisioned {
			configMapName := instance.Spec.Server.ServerTrustStoreConfigMapName
			if err != nil {
				logrus.Errorf("Error on provisioning config map '%s': %v", configMapName, err)
			} else {
				logrus.Infof("Waiting on provisioning config map '%s'", configMapName)
			}
			return reconcile.Result{}, err
		}
	}

	// Detect whether self-signed certificate is used
	selfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		logrus.Errorf("Failed to detect if self-signed certificate used. Cause: %v", err)
		return reconcile.Result{}, err
	}

	if util.IsOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if selfSignedCertUsed ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(util.IsOpenShift4 && util.IsOAuthEnabled(instance) && !instance.Spec.Server.TlsSupport) {
			if err := deploy.CreateTLSSecretFromEndpoint(deployContext, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, err
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
					return reconcile.Result{}, err
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
					return reconcile.Result{}, err
				}
			}
		}
	}

	// Make sure that CA certificates from all marked config maps are merged into single config map to be propageted to Che components
	done, err = deploy.SyncAdditionalCACertsConfigMapToCluster(deployContext)
	if err != nil {
		logrus.Errorf("Error updating additional CA config map: %v", err)
		return reconcile.Result{}, err
	}
	if !done && !tests {
		// Config map update is in progress
		// Return and do not force reconcile. When update finishes it will trigger reconcile loop.
		return reconcile.Result{}, err
	}

	if err := deploy.SetStatusDetails(deployContext, "", "", ""); err != nil {
		return reconcile.Result{}, err
	}

	// Create service account "che" for che-server component.
	// "che" is the one which token is used to create workspace objects.
	// Notice: Also we have on more "che-workspace" SA used by plugins like exec, terminal, metrics with limited privileges.
	done, err = deploy.SyncServiceAccountToCluster(deployContext, CheServiceAccountName)
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		return reconcile.Result{RequeueAfter: time.Second}, err
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
		return reconcile.Result{RequeueAfter: time.Second}, err
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
					return reconcile.Result{RequeueAfter: time.Second}, err
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
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	if err := r.GenerateAndSaveFields(deployContext, request); err != nil {
		instance, _ = r.GetCR(request)
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
	}

	if !deployContext.CheCluster.Spec.Database.ExternalDb {
		postgres := postgres.NewPostgres(deployContext)
		done, err = postgres.SyncAll()
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return reconcile.Result{}, err
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
		return reconcile.Result{}, err
	}

	// create and provision Keycloak related objects
	if !instance.Spec.Auth.ExternalIdentityProvider {
		provisioned, err := identity_provider.SyncIdentityProviderToCluster(deployContext)
		if !provisioned {
			if err != nil {
				logrus.Errorf("Error provisioning the identity provider to cluster: %v", err)
			}
			return reconcile.Result{}, err
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

	if !instance.Spec.Server.ExternalPluginRegistry {
		pluginRegistry := pluginregistry.NewPluginRegistry(deployContext)
		done, err := pluginRegistry.SyncAll()
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return reconcile.Result{}, err
		}
	} else {
		if instance.Spec.Server.PluginRegistryUrl != instance.Status.PluginRegistryURL {
			instance.Status.PluginRegistryURL = instance.Spec.Server.PluginRegistryUrl
			if err := deploy.UpdateCheCRStatus(deployContext, "status: Plugin Registry URL", instance.Spec.Server.PluginRegistryUrl); err != nil {
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
			return reconcile.Result{}, err
		}
	}

	d := dashboard.NewDashboard(deployContext)
	done, err = d.SyncAll()
	if !done {
		if err != nil {
			logrus.Errorf("Error provisioning '%s' to cluster: %v", d.GetComponentName(), err)
		}
		return reconcile.Result{}, err
	}

	err = gateway.SyncGatewayToCluster(deployContext)
	if err != nil {
		logrus.Errorf("Failed to create the Server Gateway: %s", err)
		return reconcile.Result{}, err
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
		return reconcile.Result{RequeueAfter: time.Second}, err
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
				instance, _ = r.GetCR(request)
				continue
			}
			break
		}
		for {
			instance.Spec.Auth.OAuthSecret = ""
			instance.Spec.Auth.OAuthClientName = ""
			if err := deploy.UpdateCheCRStatus(deployContext, "clean oAuth secret name and client name", ""); err != nil &&
				errors.IsConflict(err) {
				instance, _ = r.GetCR(request)
				continue
			}
			break
		}
	}

	return reconcile.Result{}, nil
}

// isTrustedBundleConfigMap detects whether given config map is the config map with additional CA certificates to be trusted by Che
func isTrustedBundleConfigMap(mgr manager.Manager, obj handler.MapObject) (bool, reconcile.Request) {
	checlusters := &orgv1.CheClusterList{}
	if err := mgr.GetClient().List(context.TODO(), checlusters, &client.ListOptions{}); err != nil {
		return false, reconcile.Request{}
	}

	if len(checlusters.Items) != 1 {
		return false, reconcile.Request{}
	}

	// Check if config map is the config map from CR
	if checlusters.Items[0].Spec.Server.ServerTrustStoreConfigMapName != obj.Meta.GetName() {
		// No, it is not form CR
		// Check for labels

		// Check for part of Che label
		if value, exists := obj.Meta.GetLabels()[deploy.KubernetesPartOfLabelKey]; !exists || value != deploy.CheEclipseOrg {
			// Labels do not match
			return false, reconcile.Request{}
		}

		// Check for CA bundle label
		if value, exists := obj.Meta.GetLabels()[deploy.CheCACertsConfigMapLabelKey]; !exists || value != deploy.CheCACertsConfigMapLabelValue {
			// Labels do not match
			return false, reconcile.Request{}
		}
	}

	return true, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: checlusters.Items[0].Namespace,
			Name:      checlusters.Items[0].Name,
		},
	}
}

func (r *ReconcileChe) autoEnableOAuth(deployContext *deploy.DeployContext, request reconcile.Request, isOpenShift4 bool) (reconcile.Result, error) {
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

// isEclipseCheSecret indicates if there is a secret with
// the label 'app.kubernetes.io/part-of=che.eclipse.org' in a che namespace
func isEclipseCheSecret(mgr manager.Manager, obj handler.MapObject) (bool, reconcile.Request) {
	checlusters := &orgv1.CheClusterList{}
	if err := mgr.GetClient().List(context.TODO(), checlusters, &client.ListOptions{}); err != nil {
		return false, reconcile.Request{}
	}

	if len(checlusters.Items) != 1 {
		return false, reconcile.Request{}
	}

	if value, exists := obj.Meta.GetLabels()[deploy.KubernetesPartOfLabelKey]; !exists || value != deploy.CheEclipseOrg {
		// Labels do not match
		return false, reconcile.Request{}
	}

	return true, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: checlusters.Items[0].Namespace,
			Name:      checlusters.Items[0].Name,
		},
	}
}

func (r *ReconcileChe) reconcileFinalizers(deployContext *deploy.DeployContext) {
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

func (r *ReconcileChe) GetCR(request reconcile.Request) (instance *orgv1.CheCluster, err error) {
	instance = &orgv1.CheCluster{}
	err = r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		logrus.Errorf("Failed to get %s CR: %s", instance.Name, err)
		return nil, err
	}
	return instance, nil
}
