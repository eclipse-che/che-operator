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
	"reflect"
	"strconv"
	"time"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
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
	return &ReconcileChe{
		client:          mgr.GetClient(),
		nonCachedClient: noncachedClient,
		scheme:          mgr.GetScheme(),
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	isOpenShift, _, err := util.DetectOpenShift()

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
	scheme          *runtime.Scheme
	tests           bool
}

const (
	failedNoOpenshiftUserReason       = "InstallOrUpdateFailed"
	warningNoIdentityProvidersMessage = "No Openshift identity providers. Openshift oAuth was disabled. How to add identity provider read in the Help Link:"
	warningNoRealUsersMessage         = "No real users. Openshift oAuth was disabled. How to add new user read in the Help Link:"
	failedUnableToGetOAuth            = "Unable to get openshift oauth."
	failedUnableToGetOpenshiftUsers   = "Unable to get users on the OpenShift cluster."

	howToAddIdentityProviderLinkOS4 = "https://docs.openshift.com/container-platform/4.1/authentication/understanding-identity-provider.html#identity-provider-overview_understanding-identity-provider"
	howToConfigureOAuthLinkOS3      = "https://docs.openshift.com/container-platform/3.11/install_config/configuring_authentication.html"
)

// Reconcile reads that state of the cluster for a CheCluster object and makes changes based on the state read
// and what is in the CheCluster.Spec. The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileChe) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	clusterAPI := deploy.ClusterAPI{
		Client: r.client,
		Scheme: r.scheme,
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

	isOpenShift, isOpenShift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("An error occurred when detecting current infra: %s", err)
	}

	// Check Che CR correctness
	if err := ValidateCheCR(instance, isOpenShift); err != nil {
		// Che cannot be deployed with current configuration.
		// Print error message in logs and wait until the configuration is changed.
		logrus.Error(err)
		return reconcile.Result{}, nil
	}

	if isOpenShift && instance.Spec.Auth.OpenShiftoAuth {
		if isOpenShift4 {
			oauthv1 := &oauthv1.OAuth{}
			if err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, oauthv1); err != nil {
				getOAuthV1ErrMsg := failedUnableToGetOAuth + " Cause: " + err.Error()
				logrus.Errorf(getOAuthV1ErrMsg)
				if err := r.SetStatusDetails(instance, request, failedNoOpenshiftUserReason, getOAuthV1ErrMsg, ""); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, err
			}
			if len(oauthv1.Spec.IdentityProviders) < 1 {
				logrus.Warn(warningNoIdentityProvidersMessage, " ", howToAddIdentityProviderLinkOS4)
				instance.Spec.Auth.OpenShiftoAuth = false
				if err := r.UpdateCheCRSpec(instance, "OpenShiftoAuth", strconv.FormatBool(false)); err != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
			}
		} else {
			users := &userv1.UserList{}
			listOptions := &client.ListOptions{}
			if err := r.nonCachedClient.List(context.TODO(), listOptions, users); err != nil {
				getUsersErrMsg := failedUnableToGetOpenshiftUsers + " Cause: " + err.Error()
				logrus.Errorf(getUsersErrMsg)
				if err := r.SetStatusDetails(instance, request, failedNoOpenshiftUserReason, getUsersErrMsg, ""); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, err
			}
			if len(users.Items) < 1 {
				logrus.Warn(warningNoRealUsersMessage, " ", howToConfigureOAuthLinkOS3)
				instance.Spec.Auth.OpenShiftoAuth = false
				if err := r.UpdateCheCRSpec(instance, "OpenShiftoAuth", strconv.FormatBool(false)); err != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
			}
		}

		// delete oAuthClient before CR is deleted
		if instance.Spec.Auth.OpenShiftoAuth {
			if err := r.ReconcileFinalizer(instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	cheFlavor := deploy.DefaultCheFlavor(instance)
	cheDeploymentName := cheFlavor

	if !isOpenShift && instance.Spec.Server.TlsSupport {
		// Ensure TLS configuration is correct
		if err := deploy.CheckAndUpdateK8sTLSConfiguration(instance, clusterAPI); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	// Detect whether self-signed certificate is used
	selfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(instance, clusterAPI)
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
			(isOpenShift4 && instance.Spec.Auth.OpenShiftoAuth && !instance.Spec.Server.TlsSupport) {
			if err := deploy.CreateTLSSecretFromRoute(instance, "", deploy.CheTLSSelfSignedCertificateSecretName, clusterAPI); err != nil {
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

		if instance.Spec.Auth.OpenShiftoAuth {
			// create a secret with OpenShift API crt to be added to keystore that RH SSO will consume
			baseURL, err := util.GetClusterPublicHostname(isOpenShift4)
			if err != nil {
				logrus.Errorf("Failed to get OpenShift cluster public hostname. A secret with API crt will not be created and consumed by RH-SSO/Keycloak")
			} else {
				if err := deploy.CreateTLSSecretFromRoute(instance, baseURL, "openshift-api-crt", clusterAPI); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	} else {
		// Handle Che TLS certificates on Kubernetes infrastructure
		if instance.Spec.Server.TlsSupport {
			result, err := deploy.K8sHandleCheTLSSecrets(instance, clusterAPI)
			if result.Requeue || result.RequeueAfter > 0 {
				if err != nil {
					logrus.Error(err)
				}
				if !tests {
					return result, err
				}
			}
		}
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
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: "devfile-registry"}, devfileRegistryConfigMap)
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
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: "plugin-registry"}, pluginRegistryConfigMap)
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
	cheSA, err := deploy.SyncServiceAccountToCluster(instance, "che", clusterAPI)
	if cheSA == nil {
		logrus.Info("Waiting on service account 'che' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWorkspaceSA, err := deploy.SyncServiceAccountToCluster(instance, "che-workspace", clusterAPI)
	if cheWorkspaceSA == nil {
		logrus.Info("Waiting on service account 'che-workspace' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// create exec and view roles for CheCluster server and workspaces
	role, err := deploy.SyncRoleToCluster(instance, "exec", []string{"pods/exec"}, []string{"*"}, clusterAPI)
	if role == nil {
		logrus.Info("Waiting on role 'exec' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	viewRole, err := deploy.SyncRoleToCluster(instance, "view", []string{"pods"}, []string{"list"}, clusterAPI)
	if viewRole == nil {
		logrus.Info("Waiting on role 'view' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheRoleBinding, err := deploy.SyncRoleBindingToCluster(instance, "che", "che", "edit", "ClusterRole", clusterAPI)
	if cheRoleBinding == nil {
		logrus.Info("Waiting on role binding 'che' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWSExecRoleBinding, err := deploy.SyncRoleBindingToCluster(instance, "che-workspace-exec", "che-workspace", "exec", "Role", clusterAPI)
	if cheWSExecRoleBinding == nil {
		logrus.Info("Waiting on role binding 'che-workspace-exec' to be created")
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWSViewRoleBinding, err := deploy.SyncRoleBindingToCluster(instance, "che-workspace-view", "che-workspace", "view", "Role", clusterAPI)
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
		cheWSCustomRoleBinding, err := deploy.SyncRoleBindingToCluster(instance, "che-workspace-custom", "view", workspaceClusterRole, "ClusterRole", clusterAPI)
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

	if err := r.GenerateAndSaveFields(instance, request, clusterAPI); err != nil {
		instance, _ = r.GetCR(request)
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
	}
	cheMultiUser := deploy.GetCheMultiUser(instance)

	if cheMultiUser == "false" {
		labels := deploy.GetLabels(instance, cheFlavor)
		pvcStatus := deploy.SyncPVCToCluster(instance, deploy.DefaultCheVolumeClaimName, "1Gi", labels, clusterAPI)
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
			if util.K8sclient.IsDeploymentExists(deploy.PostgresDeploymentName, instance.Namespace) {
				util.K8sclient.DeleteDeployment(deploy.PostgresDeploymentName, instance.Namespace)
			}
		} else {
			postgresLabels := deploy.GetLabels(instance, deploy.PostgresDeploymentName)

			// Create a new postgres service
			serviceStatus := deploy.SyncServiceToCluster(instance, "postgres", []string{"postgres"}, []int32{5432}, postgresLabels, clusterAPI)
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
			pvcStatus := deploy.SyncPVCToCluster(instance, deploy.DefaultPostgresVolumeClaimName, "1Gi", postgresLabels, clusterAPI)
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
			deploymentStatus := deploy.SyncPostgresDeploymentToCluster(instance, clusterAPI)
			if !tests {
				if !deploymentStatus.Continue {
					logrus.Infof("Waiting on deployment '%s' to be ready", deploy.PostgresDeploymentName)
					if deploymentStatus.Err != nil {
						logrus.Error(deploymentStatus.Err)
					}

					return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
				}
			}

			if !tests {
				identityProviderPostgresSecret := instance.Spec.Auth.IdentityProviderPostgresSecret
				if len(identityProviderPostgresSecret) > 0 {
					_, password, err := util.K8sclient.ReadSecret(identityProviderPostgresSecret, instance.Namespace)
					if err != nil {
						logrus.Errorf("Failed to read '%s' secret: %s", identityProviderPostgresSecret, err)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
					}
					identityProviderPostgresSecret = password
				}
				pgCommand := deploy.GetPostgresProvisionCommand(identityProviderPostgresSecret)
				dbStatus := instance.Status.DbProvisoned
				// provision Db and users for Che and Keycloak servers
				if !dbStatus {
					podToExec, err := util.K8sclient.GetDeploymentPod(deploy.PostgresDeploymentName, instance.Namespace)
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

	ingressStrategy := util.GetValue(instance.Spec.K8s.IngressStrategy, deploy.DefaultIngressStrategy)
	ingressDomain := instance.Spec.K8s.IngressDomain
	tlsSupport := instance.Spec.Server.TlsSupport
	protocol := "http"
	if tlsSupport {
		protocol = "https"
	}

	// create Che service and route
	serviceStatus := deploy.SyncCheServiceToCluster(instance, clusterAPI)
	if !tests {
		if !serviceStatus.Continue {
			logrus.Infof("Waiting on service '%s' to be ready", deploy.CheServiceHame)
			if serviceStatus.Err != nil {
				logrus.Error(serviceStatus.Err)
			}

			return reconcile.Result{Requeue: serviceStatus.Requeue}, serviceStatus.Err
		}
	}

	if !isOpenShift {
		ingressStatus := deploy.SyncIngressToCluster(instance, cheFlavor, deploy.CheIngressName, 8080, clusterAPI)
		if !tests {
			if !ingressStatus.Continue {
				logrus.Infof("Waiting on ingress '%s' to be ready", deploy.CheIngressName)
				if ingressStatus.Err != nil {
					logrus.Error(ingressStatus.Err)
				}

				return reconcile.Result{Requeue: ingressStatus.Requeue}, ingressStatus.Err
			}
		}

		cheHost := ingressDomain
		if ingressStrategy == "multi-host" {
			cheHost = cheFlavor + "-" + instance.Namespace + "." + ingressDomain
		}
		if instance.Spec.Server.CheHost != cheHost {
			instance.Spec.Server.CheHost = cheHost
			if err := r.UpdateCheCRSpec(instance, "CheHost URL", cheHost); err != nil {
				instance, _ = r.GetCR(request)
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
			}
		}
	} else {
		routeStatus := deploy.SyncRouteToCluster(instance, cheFlavor, deploy.CheRouteName, 8080, clusterAPI)
		if !tests {
			if !routeStatus.Continue {
				logrus.Infof("Waiting on route '%s' to be ready", deploy.CheRouteName)
				if routeStatus.Err != nil {
					logrus.Error(routeStatus.Err)
				}

				return reconcile.Result{Requeue: routeStatus.Requeue}, routeStatus.Err
			}

			if instance.Spec.Server.CheHost != routeStatus.Route.Spec.Host {
				instance.Spec.Server.CheHost = routeStatus.Route.Spec.Host
				if err := r.UpdateCheCRSpec(instance, "CheHost URL", instance.Spec.Server.CheHost); err != nil {
					instance, _ = r.GetCR(request)
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
			}
		}
	}

	// create and provision Keycloak related objects
	ExternalKeycloak := instance.Spec.Auth.ExternalIdentityProvider

	if !ExternalKeycloak {
		if cheMultiUser == "false" {
			if util.K8sclient.IsDeploymentExists("keycloak", instance.Namespace) {
				util.K8sclient.DeleteDeployment("keycloak", instance.Namespace)
			}
		} else {
			keycloakLabels := deploy.GetLabels(instance, "keycloak")

			serviceStatus := deploy.SyncServiceToCluster(instance, "keycloak", []string{"http"}, []int32{8080}, keycloakLabels, clusterAPI)
			if !tests {
				if !serviceStatus.Continue {
					logrus.Info("Waiting on service 'keycloak' to be ready")
					if serviceStatus.Err != nil {
						logrus.Error(serviceStatus.Err)
					}

					return reconcile.Result{Requeue: serviceStatus.Requeue}, serviceStatus.Err
				}
			}

			// create Keycloak ingresses when on k8s
			if !isOpenShift {
				ingressStatus := deploy.SyncIngressToCluster(instance, "keycloak", "keycloak", 8080, clusterAPI)
				if !tests {
					if !ingressStatus.Continue {
						logrus.Info("Waiting on ingress 'keycloak' to be ready")
						if ingressStatus.Err != nil {
							logrus.Error(ingressStatus.Err)
						}

						return reconcile.Result{Requeue: ingressStatus.Requeue}, ingressStatus.Err
					}
				}

				keycloakURL := protocol + "://" + ingressDomain
				if ingressStrategy == "multi-host" {
					keycloakURL = protocol + "://keycloak-" + instance.Namespace + "." + ingressDomain
				}
				if instance.Spec.Auth.IdentityProviderURL != keycloakURL {
					instance.Spec.Auth.IdentityProviderURL = keycloakURL
					if err := r.UpdateCheCRSpec(instance, "Keycloak URL", keycloakURL); err != nil {
						instance, _ = r.GetCR(request)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
					}
				}
			} else {
				// create Keycloak route
				routeStatus := deploy.SyncRouteToCluster(instance, "keycloak", "keycloak", 8080, clusterAPI)
				if !tests {
					if !routeStatus.Continue {
						logrus.Info("Waiting on route 'keycloak' to be ready")
						if routeStatus.Err != nil {
							logrus.Error(routeStatus.Err)
						}

						return reconcile.Result{Requeue: routeStatus.Requeue}, routeStatus.Err
					}

					keycloakURL := protocol + "://" + routeStatus.Route.Spec.Host
					if instance.Spec.Auth.IdentityProviderURL != keycloakURL {
						instance.Spec.Auth.IdentityProviderURL = keycloakURL
						if err := r.UpdateCheCRSpec(instance, "Keycloak URL", keycloakURL); err != nil {
							instance, _ = r.GetCR(request)
							return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
						}
						instance.Status.KeycloakURL = keycloakURL
						if err := r.UpdateCheCRStatus(instance, "status: Keycloak URL", keycloakURL); err != nil {
							instance, _ = r.GetCR(request)
							return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
						}
					}
				}
			}

			deploymentStatus := deploy.SyncKeycloakDeploymentToCluster(instance, clusterAPI)
			if !tests {
				if !deploymentStatus.Continue {
					logrus.Info("Waiting on deployment 'keycloak' to be ready")
					if deploymentStatus.Err != nil {
						logrus.Error(deploymentStatus.Err)
					}

					return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
				}
			}

			if !tests {
				if !instance.Status.KeycloakProvisoned {
					if err := deploy.ProvisionKeycloakResources(instance, clusterAPI); err != nil {
						logrus.Error(err)
						return reconcile.Result{RequeueAfter: time.Second}, err
					}

					for {
						instance.Status.KeycloakProvisoned = true
						if err := r.UpdateCheCRStatus(instance, "status: provisioned with Keycloak", "true"); err != nil &&
							errors.IsConflict(err) {
							instance, _ = r.GetCR(request)
							continue
						}
						break
					}
				}
			}

			if isOpenShift {
				doInstallOpenShiftoAuthProvider := instance.Spec.Auth.OpenShiftoAuth
				if doInstallOpenShiftoAuthProvider {
					openShiftIdentityProviderStatus := instance.Status.OpenShiftoAuthProvisioned
					if !openShiftIdentityProviderStatus {
						if err := r.CreateIdentityProviderItems(instance, request, cheFlavor, deploy.KeycloakDeploymentName, isOpenShift4); err != nil {
							return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
						}
					}
				}
			}
		}
	}

	// Create devfile registry resources unless an external registry is used
	devfileRegistryURL := instance.Spec.Server.DevfileRegistryUrl
	externalDevfileRegistry := instance.Spec.Server.ExternalDevfileRegistry
	if !externalDevfileRegistry {
		registryName := "devfile-registry"
		host := ""
		if !isOpenShift {
			ingressStatus := deploy.SyncIngressToCluster(instance, registryName, registryName, 8080, clusterAPI)
			if !tests {
				if !ingressStatus.Continue {
					logrus.Infof("Waiting on ingress '%s' to be ready", registryName)
					if ingressStatus.Err != nil {
						logrus.Error(ingressStatus.Err)
					}

					return reconcile.Result{Requeue: ingressStatus.Requeue}, ingressStatus.Err
				}
			}

			host = ingressDomain
			if ingressStrategy == "multi-host" {
				host = registryName + "-" + instance.Namespace + "." + ingressDomain
			}
		} else {
			routeStatus := deploy.SyncRouteToCluster(instance, registryName, registryName, 8080, clusterAPI)
			if !tests {
				if !routeStatus.Continue {
					logrus.Infof("Waiting on route '%s' to be ready", registryName)
					if routeStatus.Err != nil {
						logrus.Error(routeStatus.Err)
					}

					return reconcile.Result{Requeue: routeStatus.Requeue}, routeStatus.Err
				}
			}

			if !tests {
				host = routeStatus.Route.Spec.Host
				if len(host) < 1 {
					cheRoute := r.GetEffectiveRoute(instance, routeStatus.Route.Name)
					host = cheRoute.Spec.Host
				}
			}
		}
		guessedDevfileRegistryURL := protocol + "://" + host
		if devfileRegistryURL == "" {
			devfileRegistryURL = guessedDevfileRegistryURL
		}
		devFileRegistryConfigMap := &corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "devfile-registry", Namespace: instance.Namespace}, devFileRegistryConfigMap)
		if err != nil {
			if errors.IsNotFound(err) {
				devFileRegistryConfigMap = deploy.CreateDevfileRegistryConfigMap(instance, devfileRegistryURL)
				err = controllerutil.SetControllerReference(instance, devFileRegistryConfigMap, r.scheme)
				if err != nil {
					logrus.Errorf("An error occurred: %v", err)
					return reconcile.Result{}, err
				}
				logrus.Info("Creating devfile registry airgap configmap")
				err = r.client.Create(context.TODO(), devFileRegistryConfigMap)
				if err != nil {
					logrus.Errorf("Error creating devfile registry configmap: %v", err)
					return reconcile.Result{}, err
				}
				return reconcile.Result{Requeue: true}, nil
			} else {
				logrus.Errorf("Could not get devfile-registry ConfigMap: %v", err)
				return reconcile.Result{}, err
			}
		} else {
			newDevFileRegistryConfigMap := deploy.CreateDevfileRegistryConfigMap(instance, devfileRegistryURL)
			if !reflect.DeepEqual(devFileRegistryConfigMap.Data, newDevFileRegistryConfigMap.Data) {
				err = controllerutil.SetControllerReference(instance, devFileRegistryConfigMap, r.scheme)
				if err != nil {
					logrus.Errorf("An error occurred: %v", err)
					return reconcile.Result{}, err
				}
				logrus.Info("Updating devfile-registry ConfigMap")
				err = r.client.Update(context.TODO(), newDevFileRegistryConfigMap)
				if err != nil {
					logrus.Errorf("Error updating devfile-registry ConfigMap: %v", err)
					return reconcile.Result{}, err
				}
			}
		}

		// Create a new registry service
		registryLabels := deploy.GetLabels(instance, registryName)
		serviceStatus := deploy.SyncServiceToCluster(instance, registryName, []string{"http"}, []int32{8080}, registryLabels, clusterAPI)
		if !tests {
			if !serviceStatus.Continue {
				logrus.Info("Waiting on service '" + registryName + "' to be ready")
				if serviceStatus.Err != nil {
					logrus.Error(serviceStatus.Err)
				}

				return reconcile.Result{Requeue: serviceStatus.Requeue}, serviceStatus.Err
			}
		}

		// Deploy devfile registry
		deploymentStatus := deploy.SyncDevfileRegistryDeploymentToCluster(instance, clusterAPI)
		if !tests {
			if !deploymentStatus.Continue {
				logrus.Info("Waiting on deployment '" + registryName + "' to be ready")
				if deploymentStatus.Err != nil {
					logrus.Error(deploymentStatus.Err)
				}

				return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
			}
		}
	}
	if devfileRegistryURL != instance.Status.DevfileRegistryURL {
		instance.Status.DevfileRegistryURL = devfileRegistryURL
		if err := r.UpdateCheCRStatus(instance, "status: Devfile Registry URL", devfileRegistryURL); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	pluginRegistryURL := instance.Spec.Server.PluginRegistryUrl
	// Create Plugin registry resources unless an external registry is used
	externalPluginRegistry := instance.Spec.Server.ExternalPluginRegistry
	if !externalPluginRegistry {
		if instance.IsAirGapMode() {
			pluginRegistryConfigMap := &corev1.ConfigMap{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "plugin-registry", Namespace: instance.Namespace}, pluginRegistryConfigMap)
			if err != nil {
				if errors.IsNotFound(err) {
					pluginRegistryConfigMap = deploy.CreatePluginRegistryConfigMap(instance)
					err = controllerutil.SetControllerReference(instance, pluginRegistryConfigMap, r.scheme)
					if err != nil {
						logrus.Errorf("An error occurred: %v", err)
						return reconcile.Result{}, err
					}
					logrus.Info("Creating plugin registry airgap configmap")
					err = r.client.Create(context.TODO(), pluginRegistryConfigMap)
					if err != nil {
						logrus.Errorf("Error creating plugin registry configmap: %v", err)
						return reconcile.Result{}, err
					}
					return reconcile.Result{Requeue: true}, nil
				} else {
					logrus.Errorf("Could not get plugin-registry ConfigMap: %v", err)
					return reconcile.Result{}, err
				}
			} else {
				pluginRegistryConfigMap = deploy.CreatePluginRegistryConfigMap(instance)
				err = controllerutil.SetControllerReference(instance, pluginRegistryConfigMap, r.scheme)
				if err != nil {
					logrus.Errorf("An error occurred: %v", err)
					return reconcile.Result{}, err
				}
				logrus.Info(" Updating plugin-registry ConfigMap")
				err = r.client.Update(context.TODO(), pluginRegistryConfigMap)
				if err != nil {
					logrus.Errorf("Error updating plugin-registry ConfigMap: %v", err)
					return reconcile.Result{}, err
				}
			}
		}

		registryName := "plugin-registry"
		host := ""
		if !isOpenShift {
			ingressStatus := deploy.SyncIngressToCluster(instance, registryName, registryName, 8080, clusterAPI)
			if !tests {
				if !ingressStatus.Continue {
					logrus.Infof("Waiting on ingress '%s' to be ready", registryName)
					if ingressStatus.Err != nil {
						logrus.Error(ingressStatus.Err)
					}

					return reconcile.Result{Requeue: ingressStatus.Requeue}, ingressStatus.Err
				}
			}

			host = ingressDomain
			if ingressStrategy == "multi-host" {
				host = registryName + "-" + instance.Namespace + "." + ingressDomain
			}
		} else {
			routeStatus := deploy.SyncRouteToCluster(instance, registryName, registryName, 8080, clusterAPI)
			if !tests {
				if !routeStatus.Continue {
					logrus.Infof("Waiting on route '%s' to be ready", registryName)
					if routeStatus.Err != nil {
						logrus.Error(routeStatus.Err)
					}

					return reconcile.Result{Requeue: routeStatus.Requeue}, routeStatus.Err
				}
			}

			if !tests {
				host = routeStatus.Route.Spec.Host
			}
		}

		guessedPluginRegistryURL := protocol + "://" + host
		guessedPluginRegistryURL += "/v3"
		if pluginRegistryURL == "" {
			pluginRegistryURL = guessedPluginRegistryURL
		}

		// Create a new registry service
		registryLabels := deploy.GetLabels(instance, registryName)
		serviceStatus := deploy.SyncServiceToCluster(instance, registryName, []string{"http"}, []int32{8080}, registryLabels, clusterAPI)
		if !tests {
			if !serviceStatus.Continue {
				logrus.Info("Waiting on service '" + registryName + "' to be ready")
				if serviceStatus.Err != nil {
					logrus.Error(serviceStatus.Err)
				}

				return reconcile.Result{Requeue: serviceStatus.Requeue}, serviceStatus.Err
			}
		}

		// Deploy plugin registry
		deploymentStatus := deploy.SyncPluginRegistryDeploymentToCluster(instance, clusterAPI)
		if !tests {
			if !deploymentStatus.Continue {
				logrus.Info("Waiting on deployment '" + registryName + "' to be ready")
				if deploymentStatus.Err != nil {
					logrus.Error(deploymentStatus.Err)
				}

				return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
			}
		}
	}
	if pluginRegistryURL != instance.Status.PluginRegistryURL {
		instance.Status.PluginRegistryURL = pluginRegistryURL
		if err := r.UpdateCheCRStatus(instance, "status: Plugin Registry URL", pluginRegistryURL); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	if serverTrustStoreConfigMapName := instance.Spec.Server.ServerTrustStoreConfigMapName; serverTrustStoreConfigMapName != "" {
		certMap := r.GetEffectiveConfigMap(instance, serverTrustStoreConfigMapName)
		if err := controllerutil.SetControllerReference(instance, certMap, r.scheme); err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
	}

	// create Che ConfigMap which is synced with CR and is not supposed to be manually edited
	// controller will reconcile this CM with CR spec
	cheEnv := deploy.GetConfigMapData(instance)
	configMapStatus := deploy.SyncConfigMapToCluster(instance, cheEnv, clusterAPI)
	if !tests {
		if !configMapStatus.Continue {
			logrus.Infof("Waiting on config map '%s' to be created", cheFlavor)
			if configMapStatus.Err != nil {
				logrus.Error(configMapStatus.Err)
			}

			return reconcile.Result{Requeue: configMapStatus.Requeue}, configMapStatus.Err
		}
	}

	// configMap resource version will be an env in Che deployment to easily update it when a ConfigMap changes
	// which will automatically trigger Che rolling update
	var cmResourceVersion string
	if tests {
		cmResourceVersion = r.GetEffectiveConfigMap(instance, deploy.CheConfigMapName).ResourceVersion
	} else {
		cmResourceVersion = configMapStatus.ConfigMap.ResourceVersion
	}

	// Create a new che deployment
	deploymentStatus := deploy.SyncCheDeploymentToCluster(instance, cmResourceVersion, clusterAPI)
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
