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
	"fmt"
	"strconv"
	"strings"
	"time"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	devfile_registry "github.com/eclipse/che-operator/pkg/deploy/devfile-registry"
	"github.com/eclipse/che-operator/pkg/deploy/gateway"
	identity_provider "github.com/eclipse/che-operator/pkg/deploy/identity-provider"
	plugin_registry "github.com/eclipse/che-operator/pkg/deploy/plugin-registry"
	"github.com/eclipse/che-operator/pkg/deploy/postgres"
	"github.com/eclipse/che-operator/pkg/deploy/server"
	"github.com/eclipse/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		client:          mgr.GetClient(),
		nonCachedClient: noncachedClient,
		scheme:          mgr.GetScheme(),
		discoveryClient: discoveryClient,
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	isOpenShift, _, err := util.DetectOpenShift()

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

	if err != nil {
		logrus.Errorf("An error occurred when detecting current infra: %s", err)
	}
	// Create a new controller
	c, err := controller.New("che-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// register OpenShift specific types in the scheme
	if isOpenShift {
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
		if hasConsolelinkObject() {
			if err := consolev1.AddToScheme(mgr.GetScheme()); err != nil {
				logrus.Errorf("Failed to add OpenShift ConsoleLink to scheme: %s", err)
			}
		}
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

	if isOpenShift {
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

var _ reconcile.Reconciler = &ReconcileChe{}
var oAuthFinalizerName = "oauthclients.finalizers.che.eclipse.org"

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
	discoveryClient discovery.DiscoveryInterface
	scheme          *runtime.Scheme
	tests           bool
}

const (
	failedValidationReason            = "InstallOrUpdateFailed"
	failedNoOpenshiftUserReason       = "InstallOrUpdateFailed"
	warningNoIdentityProvidersMessage = "No Openshift identity providers. Openshift oAuth was disabled. How to add identity provider read in the Help Link:"
	warningNoRealUsersMessage         = "No real users. Openshift oAuth was disabled. How to add new user read in the Help Link:"
	failedUnableToGetOAuth            = "Unable to get openshift oauth."
	failedUnableToGetOpenshiftUsers   = "Unable to get users on the OpenShift cluster."

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
		ClusterAPI:      clusterAPI,
		CheCluster:      instance,
		InternalService: deploy.InternalService{},
	}

	// delete oAuthClient before CR is deleted
	// todo check
	// instance.Status.OpenShiftoAuthProvisioned
	if util.IsOpenShift && util.IsOAuthEnabled(instance) {
		if err := r.ReconcileFinalizer(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Reconcile the imagePuller section of the CheCluster
	imagePullerResult, err := deploy.ReconcileImagePuller(deployContext)
	if err != nil {
		return imagePullerResult, err
	}
	if imagePullerResult.Requeue || imagePullerResult.RequeueAfter > 0 {
		return imagePullerResult, err
	}

	isOpenShift, isOpenShift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("An error occurred when detecting current infra: %s", err)
	}

	// Check Che CR correctness
	if err := ValidateCheCR(instance, isOpenShift); err != nil {
		// Che cannot be deployed with current configuration.
		// Print error message in logs and wait until the configuration is changed.
		logrus.Error(err)
		if err := r.SetStatusDetails(instance, request, failedValidationReason, err.Error(), ""); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if !util.IsTestMode() {
		if isOpenShift && deployContext.DefaultCheHost == "" {
			host, err := getDefaultCheHost(deployContext)
			if host == "" {
				return reconcile.Result{RequeueAfter: 1 * time.Second}, err
			}
			deployContext.DefaultCheHost = host
		}
	}

	if isOpenShift && instance.Spec.Auth.OpenShiftoAuth == nil {
		if reconcileResult, err := r.autoEnableOAuth(instance, request, isOpenShift4); err != nil {
			return reconcileResult, err
		}
	}

	// Read proxy configuration
	proxy, err := r.getProxyConfiguration(instance)
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

	cheFlavor := deploy.DefaultCheFlavor(instance)
	cheDeploymentName := cheFlavor

	// Detect whether self-signed certificate is used
	selfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		logrus.Errorf("Failed to detect if self-signed certificate used. Cause: %v", err)
		return reconcile.Result{}, err
	}

	if isOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if selfSignedCertUsed ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(isOpenShift4 && util.IsOAuthEnabled(instance) && !instance.Spec.Server.TlsSupport) {
			if err := deploy.CreateTLSSecretFromEndpoint(deployContext, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, err
			}
		}

		if !tests {
			deployment := &appsv1.Deployment{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: cheDeploymentName, Namespace: instance.Namespace}, deployment)
			if err != nil && instance.Status.CheClusterRunning != UnavailableStatus {
				if err := r.SetCheUnavailableStatus(instance, request); err != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
			}
		}

		if util.IsOAuthEnabled(instance) {
			// create a secret with OpenShift API crt to be added to keystore that RH SSO will consume
			baseURL, err := util.GetClusterPublicHostname(isOpenShift4)
			if err != nil {
				logrus.Errorf("Failed to get OpenShift cluster public hostname. A secret with API crt will not be created and consumed by RH-SSO/Keycloak")
			} else {
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
	cm, err := deploy.SyncAdditionalCACertsConfigMapToCluster(deployContext)
	if err != nil {
		logrus.Errorf("Error updating additional CA config map: %v", err)
		return reconcile.Result{}, err
	}
	if cm == nil && !tests {
		// Config map update is in progress
		// Return and do not force reconcile. When update finishes it will trigger reconcile loop.
		return reconcile.Result{}, err
	}

	// Get custom ConfigMap
	// if it exists, add the data into CustomCheProperties
	customConfigMap := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: "custom"}, customConfigMap)
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("Error getting custom configMap: %v", err)
		return reconcile.Result{}, err
	}
	if err == nil {
		logrus.Infof("Found legacy custom ConfigMap.  Adding those values to CheCluster.Spec.Server.CustomCheProperties")
		if instance.Spec.Server.CustomCheProperties == nil {
			instance.Spec.Server.CustomCheProperties = make(map[string]string)
		}
		for k, v := range customConfigMap.Data {
			instance.Spec.Server.CustomCheProperties[k] = v
		}
		if err := r.client.Update(context.TODO(), instance); err != nil {
			logrus.Errorf("Error updating CheCluster: %v", err)
			return reconcile.Result{}, err
		}
		if err = r.client.Delete(context.TODO(), customConfigMap); err != nil {
			logrus.Errorf("Error deleting legacy custom ConfigMap: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// If the devfile-registry ConfigMap exists, and we are not in airgapped mode, delete the ConfigMap
	devfileRegistryConfigMap := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: deploy.DevfileRegistryName}, devfileRegistryConfigMap)
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("Error getting devfile-registry ConfigMap: %v", err)
		return reconcile.Result{}, err
	}
	if err == nil && instance.Spec.Server.ExternalDevfileRegistry {
		logrus.Info("Found devfile-registry ConfigMap and while using an external devfile registry.  Deleting.")
		if err = r.client.Delete(context.TODO(), devfileRegistryConfigMap); err != nil {
			logrus.Errorf("Error deleting devfile-registry ConfigMap: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// If the plugin-registry ConfigMap exists, and we are not in airgapped mode, delete the ConfigMap
	pluginRegistryConfigMap := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: deploy.PluginRegistryName}, pluginRegistryConfigMap)
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("Error getting plugin-registry ConfigMap: %v", err)
		return reconcile.Result{}, err
	}
	if err == nil && !instance.IsAirGapMode() {
		logrus.Info("Found plugin-registry ConfigMap and not in airgap mode.  Deleting.")
		if err = r.client.Delete(context.TODO(), pluginRegistryConfigMap); err != nil {
			logrus.Errorf("Error deleting plugin-registry ConfigMap: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	if err := r.SetStatusDetails(instance, request, "", "", ""); err != nil {
		return reconcile.Result{}, err
	}

	// create service accounts:
	// che is the one which token is used to create workspace objects
	// che-workspace is SA used by plugins like exec and terminal with limited privileges
	cheSA, err := deploy.SyncServiceAccountToCluster(deployContext, "che")
	if cheSA == nil {
		logrus.Info("Waiting on service account 'che' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWorkspaceSA, err := deploy.SyncServiceAccountToCluster(deployContext, "che-workspace")
	if cheWorkspaceSA == nil {
		logrus.Info("Waiting on service account 'che-workspace' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// create exec role for CheCluster server and workspaces
	execRole, err := deploy.SyncExecRoleToCluster(deployContext)
	if execRole == nil {
		logrus.Info("Waiting on role 'exec' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// create view role for CheCluster server and workspaces
	viewRole, err := deploy.SyncViewRoleToCluster(deployContext)
	if viewRole == nil {
		logrus.Info("Waiting on role 'view' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, "che", "che", "edit", "ClusterRole")
	if cheRoleBinding == nil {
		logrus.Info("Waiting on role binding 'che' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	if len(instance.Spec.Server.CheClusterRoles) > 0 {
		cheClusterRoles := strings.Split(instance.Spec.Server.CheClusterRoles, ",")
		for _, cheClusterRole := range cheClusterRoles {
			cheClusterRole := strings.TrimSpace(cheClusterRole)
			cheClusterRoleBindingName := instance.Namespace + "-che-" + cheClusterRole
			cheClusterRoleBinding, err := deploy.SyncClusterRoleBindingToCluster(deployContext, cheClusterRoleBindingName, "che", cheClusterRole)
			if cheClusterRoleBinding == nil {
				logrus.Infof("Waiting on cluster role binding '%s' to be created", cheClusterRoleBindingName)
				if err != nil {
					logrus.Error(err)
				}
				if !tests {
					return reconcile.Result{RequeueAfter: time.Second}, err
				}
			}
		}
	}

	cheWSExecRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, "che-workspace-exec", "che-workspace", "exec", "Role")
	if cheWSExecRoleBinding == nil {
		logrus.Info("Waiting on role binding 'che-workspace-exec' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWSViewRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, "che-workspace-view", "che-workspace", "view", "Role")
	if cheWSViewRoleBinding == nil {
		logrus.Info("Waiting on role binding 'che-workspace-view' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// If the user specified an additional cluster role to use for the Che workspace, create a role binding for it
	// Use a role binding instead of a cluster role binding to keep the additional access scoped to the workspace's namespace
	workspaceClusterRole := instance.Spec.Server.CheWorkspaceClusterRole
	if workspaceClusterRole != "" {
		cheWSCustomRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, "che-workspace-custom", "view", workspaceClusterRole, "ClusterRole")
		if cheWSCustomRoleBinding == nil {
			logrus.Info("Waiting on role binding 'che-workspace-custom' to be created")
			if err != nil {
				logrus.Error(err)
			}
			if !tests {
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
		}
	}

	if err := r.GenerateAndSaveFields(deployContext, request); err != nil {
		instance, _ = r.GetCR(request)
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
	}
	cheMultiUser := deploy.GetCheMultiUser(instance)

	if cheMultiUser == "false" {
		labels := deploy.GetLabels(instance, cheFlavor)
		pvcStatus := deploy.SyncPVCToCluster(deployContext, deploy.DefaultCheVolumeClaimName, "1Gi", labels)
		if !tests {
			if !pvcStatus.Continue {
				logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", deploy.DefaultCheVolumeClaimName)
				if pvcStatus.Err != nil {
					logrus.Error(pvcStatus.Err)
				}
				return reconcile.Result{Requeue: pvcStatus.Requeue, RequeueAfter: time.Second * 1}, pvcStatus.Err
			}
		}

		if util.K8sclient.IsPVCExists(deploy.DefaultPostgresVolumeClaimName, instance.Namespace) {
			util.K8sclient.DeletePVC(deploy.DefaultPostgresVolumeClaimName, instance.Namespace)
		}
	} else {
		if !tests {
			if util.K8sclient.IsPVCExists(deploy.DefaultCheVolumeClaimName, instance.Namespace) {
				util.K8sclient.DeletePVC(deploy.DefaultCheVolumeClaimName, instance.Namespace)
			}
		}
	}

	// Create Postgres resources and provisioning unless an external DB is used
	externalDB := instance.Spec.Database.ExternalDb
	if !externalDB {
		if cheMultiUser == "false" {
			if util.K8sclient.IsDeploymentExists(deploy.PostgresName, instance.Namespace) {
				util.K8sclient.DeleteDeployment(deploy.PostgresName, instance.Namespace)
			}
		} else {
			postgresLabels := deploy.GetLabels(instance, deploy.PostgresName)

			// Create a new postgres service
			serviceStatus := deploy.SyncServiceToCluster(deployContext, deploy.PostgresName, []string{deploy.PostgresName}, []int32{5432}, postgresLabels)
			if !tests {
				if !serviceStatus.Continue {
					logrus.Info("Waiting on service 'postgres' to be ready")
					if serviceStatus.Err != nil {
						logrus.Error(serviceStatus.Err)
					}

					return reconcile.Result{Requeue: serviceStatus.Requeue}, serviceStatus.Err
				}
			}

			// Create a new Postgres PVC object
			pvcStatus := deploy.SyncPVCToCluster(deployContext, deploy.DefaultPostgresVolumeClaimName, "1Gi", postgresLabels)
			if !tests {
				if !pvcStatus.Continue {
					logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", deploy.DefaultPostgresVolumeClaimName)
					if pvcStatus.Err != nil {
						logrus.Error(pvcStatus.Err)
					}

					return reconcile.Result{Requeue: pvcStatus.Requeue, RequeueAfter: time.Second * 1}, pvcStatus.Err
				}
			}

			// Create a new Postgres deployment
			deploymentStatus := postgres.SyncPostgresDeploymentToCluster(deployContext)
			if !tests {
				if !deploymentStatus.Continue {
					logrus.Infof("Waiting on deployment '%s' to be ready", deploy.PostgresName)
					if deploymentStatus.Err != nil {
						logrus.Error(deploymentStatus.Err)
					}

					return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
				}
			}

			if !tests {
				identityProviderPostgresPassword := instance.Spec.Auth.IdentityProviderPostgresPassword
				identityProviderPostgresSecret := instance.Spec.Auth.IdentityProviderPostgresSecret
				if len(identityProviderPostgresSecret) > 0 {
					_, password, err := util.K8sclient.ReadSecret(identityProviderPostgresSecret, instance.Namespace)
					if err != nil {
						logrus.Errorf("Failed to read '%s' secret: %s", identityProviderPostgresSecret, err)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
					}
					identityProviderPostgresPassword = password
				}
				pgCommand := identity_provider.GetPostgresProvisionCommand(identityProviderPostgresPassword)
				dbStatus := instance.Status.DbProvisoned
				// provision Db and users for Che and Keycloak servers
				if !dbStatus {
					podToExec, err := util.K8sclient.GetDeploymentPod(deploy.PostgresName, instance.Namespace)
					if err != nil {
						return reconcile.Result{}, err
					}
					_, err = util.K8sclient.ExecIntoPod(podToExec, pgCommand, "create Keycloak DB, user, privileges", instance.Namespace)
					if err == nil {
						for {
							instance.Status.DbProvisoned = true
							if err := r.UpdateCheCRStatus(instance, "status: provisioned with DB and user", "true"); err != nil &&
								errors.IsConflict(err) {
								instance, _ = r.GetCR(request)
								continue
							}
							break
						}
					} else {
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
					}
				}
			}
		}
	}

	tlsSupport := instance.Spec.Server.TlsSupport
	protocol := "http"
	if tlsSupport {
		protocol = "https"
	}

	// create Che service and route
	serviceStatus := server.SyncCheServiceToCluster(deployContext)
	if !tests {
		if !serviceStatus.Continue {
			logrus.Infof("Waiting on service '%s' to be ready", deploy.CheServiceName)
			if serviceStatus.Err != nil {
				logrus.Error(serviceStatus.Err)
			}

			return reconcile.Result{Requeue: serviceStatus.Requeue}, serviceStatus.Err
		}
	}

	deployContext.InternalService.CheHost = fmt.Sprintf("http://%s.%s.svc:8080", deploy.CheServiceName, deployContext.CheCluster.Namespace)

	exposedServiceName := getServerExposingServiceName(instance)
	cheHost := ""
	if !isOpenShift {
		additionalLabels := deployContext.CheCluster.Spec.Server.CheServerIngress.Labels
		ingress, err := deploy.SyncIngressToCluster(deployContext, cheFlavor, instance.Spec.Server.CheHost, exposedServiceName, 8080, additionalLabels)
		if !tests {
			if ingress == nil {
				logrus.Infof("Waiting on ingress '%s' to be ready", cheFlavor)
				if err != nil {
					logrus.Error(err)
				}

				return reconcile.Result{RequeueAfter: time.Second * 1}, err
			}
			cheHost = ingress.Spec.Rules[0].Host
		}
	} else {
		customHost := instance.Spec.Server.CheHost
		if deployContext.DefaultCheHost == customHost {
			// let OpenShift set a hostname by itself since it requires a routes/custom-host permissions
			customHost = ""
		}

		additionalLabels := deployContext.CheCluster.Spec.Server.CheServerRoute.Labels
		route, err := deploy.SyncRouteToCluster(deployContext, cheFlavor, customHost, exposedServiceName, 8080, additionalLabels)
		if route == nil {
			logrus.Infof("Waiting on route '%s' to be ready", cheFlavor)
			if err != nil {
				logrus.Error(err)
			}

			return reconcile.Result{RequeueAfter: time.Second * 1}, err
		}
		cheHost = route.Spec.Host
		if customHost == "" {
			deployContext.DefaultCheHost = cheHost
		}
	}
	if instance.Spec.Server.CheHost != cheHost {
		instance.Spec.Server.CheHost = cheHost
		if err := r.UpdateCheCRSpec(instance, "CheHost URL", cheHost); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	// create and provision Keycloak related objects
	provisioned, err := identity_provider.SyncIdentityProviderToCluster(deployContext)
	if !tests {
		if !provisioned {
			if err != nil {
				logrus.Errorf("Error provisioning the identity provider to cluster: %v", err)
			}
			return reconcile.Result{RequeueAfter: time.Second * 1}, err
		}
	}

	provisioned, err = devfile_registry.SyncDevfileRegistryToCluster(deployContext, cheHost)
	if !tests {
		if !provisioned {
			if err != nil {
				logrus.Errorf("Error provisioning '%s' to cluster: %v", deploy.DevfileRegistryName, err)
			}
			return reconcile.Result{RequeueAfter: time.Second * 1}, err
		}
	}

	provisioned, err = plugin_registry.SyncPluginRegistryToCluster(deployContext, cheHost)
	if !tests {
		if !provisioned {
			if err != nil {
				logrus.Errorf("Error provisioning '%s' to cluster: %v", deploy.PluginRegistryName, err)
			}
			return reconcile.Result{RequeueAfter: time.Second * 1}, err
		}
	}

	// create Che ConfigMap which is synced with CR and is not supposed to be manually edited
	// controller will reconcile this CM with CR spec
	cheConfigMap, err := server.SyncCheConfigMapToCluster(deployContext)
	if !tests {
		if cheConfigMap == nil {
			logrus.Infof("Waiting on config map '%s' to be created", server.CheConfigMapName)
			if err != nil {
				logrus.Error(err)
			}
			return reconcile.Result{}, err
		}
	}

	err = gateway.SyncGatewayToCluster(deployContext)
	if err != nil {
		logrus.Errorf("Failed to create the Server Gateway: %s", err)
		return reconcile.Result{}, err
	}

	// Create a new che deployment
	deploymentStatus := server.SyncCheDeploymentToCluster(deployContext)
	if !tests {
		if !deploymentStatus.Continue {
			logrus.Infof("Waiting on deployment '%s' to be ready", cheFlavor)
			if deploymentStatus.Err != nil {
				logrus.Error(deploymentStatus.Err)
			}

			deployment, err := r.GetEffectiveDeployment(instance, cheFlavor)
			if err == nil {
				if deployment.Status.AvailableReplicas < 1 {
					if instance.Status.CheClusterRunning != UnavailableStatus {
						if err := r.SetCheUnavailableStatus(instance, request); err != nil {
							instance, _ = r.GetCR(request)
							return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
						}
					}
				} else if deployment.Status.Replicas != 1 {
					if instance.Status.CheClusterRunning != RollingUpdateInProgressStatus {
						if err := r.SetCheRollingUpdateStatus(instance, request); err != nil {
							instance, _ = r.GetCR(request)
							return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
						}
					}
				}
			}
			return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
		}
	}
	// Update available status
	if instance.Status.CheClusterRunning != AvailableStatus {
		cheHost := instance.Spec.Server.CheHost
		if err := r.SetCheAvailableStatus(instance, request, protocol, cheHost); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	// Update Che version status
	cheVersion := EvaluateCheServerVersion(instance)
	if instance.Status.CheVersion != cheVersion {
		instance.Status.CheVersion = cheVersion
		if err := r.UpdateCheCRStatus(instance, "version", cheVersion); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	// we can now try to create consolelink, after che instance is available
	if err := createConsoleLink(isOpenShift4, protocol, instance, r); err != nil {
		logrus.Errorf("An error occurred during console link provisioning: %s", err)
		return reconcile.Result{}, err
	}

	// Delete OpenShift identity provider if OpenShift oAuth is false in spec
	// but OpenShiftoAuthProvisioned is true in CR status, e.g. when oAuth has been turned on and then turned off
	deleted, err := r.ReconcileIdentityProvider(instance, isOpenShift4)
	if deleted {
		for {
			if err := r.DeleteFinalizer(instance); err != nil &&
				errors.IsConflict(err) {
				instance, _ = r.GetCR(request)
				continue
			}
			break
		}
		for {
			instance.Status.OpenShiftoAuthProvisioned = false
			if err := r.UpdateCheCRStatus(instance, "status: provisioned with OpenShift identity provider", "false"); err != nil &&
				errors.IsConflict(err) {
				instance, _ = r.GetCR(request)
				continue
			}
			break
		}
		for {
			instance.Spec.Auth.OAuthSecret = ""
			instance.Spec.Auth.OAuthClientName = ""
			if err := r.UpdateCheCRSpec(instance, "clean oAuth secret name and client name", ""); err != nil &&
				errors.IsConflict(err) {
				instance, _ = r.GetCR(request)
				continue
			}
			break
		}
	}

	return reconcile.Result{}, nil
}

func createConsoleLink(isOpenShift4 bool, protocol string, instance *orgv1.CheCluster, r *ReconcileChe) error {
	if !isOpenShift4 || !hasConsolelinkObject() {
		logrus.Debug("Console link won't be created. It's not supported by cluster")
		// console link is supported only on OpenShift >= 4.2
		return nil
	}

	if protocol != "https" {
		logrus.Debug("Console link won't be created. It's not supported when http connection is used")
		// console link is supported only with https
		return nil
	}
	cheHost := instance.Spec.Server.CheHost
	preparedConsoleLink := &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploy.DefaultConsoleLinkName(),
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: instance.Namespace,
			},
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: protocol + "://" + cheHost,
				Text: deploy.DefaultConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  deploy.DefaultConsoleLinkSection(),
				ImageURL: fmt.Sprintf("%s://%s%s", protocol, cheHost, deploy.DefaultConsoleLinkImage()),
			},
		},
	}

	existingConsoleLink := &consolev1.ConsoleLink{}

	if getErr := r.nonCachedClient.Get(context.TODO(), client.ObjectKey{Name: deploy.DefaultConsoleLinkName()}, existingConsoleLink); getErr == nil {
		// if found, update existing one. We need ResourceVersion from current one.
		preparedConsoleLink.ResourceVersion = existingConsoleLink.ResourceVersion
		logrus.Debugf("Updating the object: ConsoleLink, name: %s", existingConsoleLink.Name)
		return r.nonCachedClient.Update(context.TODO(), preparedConsoleLink)
	} else {
		// if not found, create new one
		if statusError, ok := getErr.(*errors.StatusError); ok &&
			statusError.Status().Reason == metav1.StatusReasonNotFound {
			logrus.Infof("Creating a new object: ConsoleLink, name: %s", preparedConsoleLink.Name)
			return r.nonCachedClient.Create(context.TODO(), preparedConsoleLink)
		} else {
			return getErr
		}
	}
}

func hasConsolelinkObject() bool {
	resourceList, err := util.GetServerResources()
	if err != nil {
		return false
	}
	for _, res := range resourceList {
		for _, r := range res.APIResources {
			if r.Name == "consolelinks" {
				return true
			}
		}
	}
	return false
}

// EvaluateCheServerVersion evaluate che version
// based on Checluster information and image defaults from env variables
func EvaluateCheServerVersion(cr *orgv1.CheCluster) string {
	return util.GetValue(cr.Spec.Server.CheImageTag, deploy.DefaultCheVersion())
}

func getDefaultCheHost(deployContext *deploy.DeployContext) (string, error) {
	routeName := deploy.DefaultCheFlavor(deployContext.CheCluster)
	additionalLabels := deployContext.CheCluster.Spec.Server.CheServerRoute.Labels
	route, err := deploy.SyncRouteToCluster(deployContext, routeName, "", getServerExposingServiceName(deployContext.CheCluster), 8080, additionalLabels)
	if route == nil {
		logrus.Infof("Waiting on route '%s' to be ready", routeName)
		if err != nil {
			logrus.Error(err)
		}
		return "", err
	}
	return route.Spec.Host, nil
}

func getServerExposingServiceName(cr *orgv1.CheCluster) string {
	if cr.Spec.Server.ServerExposureStrategy == "single-host" && deploy.GetSingleHostExposureType(cr) == "gateway" {
		return gateway.GatewayServiceName
	}
	return deploy.CheServiceName
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

func (r *ReconcileChe) autoEnableOAuth(cr *orgv1.CheCluster, request reconcile.Request, isOpenShift4 bool) (reconcile.Result, error) {
	var message, reason string
	oauth := false
	if isOpenShift4 {
		oauthv1 := &oauthv1.OAuth{}
		if err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, oauthv1); err != nil {
			getOAuthV1ErrMsg := failedUnableToGetOAuth + " Cause: " + err.Error()
			logrus.Errorf(getOAuthV1ErrMsg)
			message = getOAuthV1ErrMsg
			reason = failedNoOpenshiftUserReason
		} else {
			oauth = len(oauthv1.Spec.IdentityProviders) >= 1
			if !oauth {
				logrus.Warn(warningNoIdentityProvidersMessage, " ", howToAddIdentityProviderLinkOS4)
			}
		}
		// openshift 3
	} else {
		users := &userv1.UserList{}
		listOptions := &client.ListOptions{}
		if err := r.nonCachedClient.List(context.TODO(), users, listOptions); err != nil {
			getUsersErrMsg := failedUnableToGetOpenshiftUsers + " Cause: " + err.Error()
			logrus.Errorf(getUsersErrMsg)
			message = getUsersErrMsg
			reason = failedNoOpenshiftUserReason
		} else {
			oauth = len(users.Items) >= 1
			if !oauth {
				logrus.Warn(warningNoRealUsersMessage, " ", howToConfigureOAuthLinkOS3)
			}
		}
	}

	cr.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(oauth)
	if err := r.UpdateCheCRSpec(cr, "OpenShiftoAuth", strconv.FormatBool(oauth)); err != nil {
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
	}

	if message != "" && reason != "" {
		if err := r.SetStatusDetails(cr, request, message, reason, ""); err != nil {
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
