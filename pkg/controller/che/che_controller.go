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
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	consolev1 "github.com/openshift/api/console/v1"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"
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
var (
	k8sclient = util.GetK8Client()
)

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
	failedNoOpenshiftUserReason  = "InstallOrUpdateFailed"
	failedNoOpenshiftUserMessage = "No real user exists in the OpenShift cluster." +
		" Either disable OpenShift OAuth integration or add at least one user (details in the Help link)"
	failedUnableToGetOpenshiftUsers = "Unable to get users on the OpenShift cluster."
	howToCreateAUserLinkOS4         = "https://docs.openshift.com/container-platform/4.1/authentication/understanding-identity-provider.html#identity-provider-overview_understanding-identity-provider"
	howToCreateAUserLinkOS3         = "https://docs.openshift.com/container-platform/3.11/install_config/configuring_authentication.html"
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

	if isOpenShift {
		// delete oAuthClient before CR is deleted
		doInstallOpenShiftoAuthProvider := instance.Spec.Auth.OpenShiftoAuth
		if doInstallOpenShiftoAuthProvider {
			if err := r.ReconcileFinalizer(instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Handle Che TLS certificates on Kubernetes like infrastructures
	if instance.Spec.Server.TlsSupport && instance.Spec.Server.SelfSignedCert && !isOpenShift {
		shouldReturn, reconsileResult, err := HandleCheTLSSecrets(instance, r)
		if shouldReturn {
			return reconsileResult, err
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

	cheFlavor := util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor)
	cheDeploymentName := cheFlavor
	if isOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if instance.Spec.Server.SelfSignedCert ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(isOpenShift4 && instance.Spec.Auth.OpenShiftoAuth && !instance.Spec.Server.TlsSupport) {
			if err := r.CreateTLSSecret(instance, "", "self-signed-certificate", clusterAPI); err != nil {
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
				helpLink := ""
				if isOpenShift4 {
					helpLink = howToCreateAUserLinkOS4
				} else {
					helpLink = howToCreateAUserLinkOS3
				}
				logrus.Errorf(failedNoOpenshiftUserMessage)
				if err := r.SetStatusDetails(instance, request, failedNoOpenshiftUserReason, failedNoOpenshiftUserMessage, helpLink); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
			}

			// create a secret with OpenShift API crt to be added to keystore that RH SSO will consume
			baseURL, err := util.GetClusterPublicHostname(isOpenShift4)
			if err != nil {
				logrus.Errorf("Failed to get OpenShift cluster public hostname. A secret with API crt will not be created and consumed by RH-SSO/Keycloak")
			} else {
				if err := r.CreateTLSSecret(instance, baseURL, "openshift-api-crt", clusterAPI); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}

	if err := r.SetStatusDetails(instance, request, "", "", ""); err != nil {
		return reconcile.Result{}, err
	}

	// create service accounts:
	// che is the one which token is used to create workspace objects
	// che-workspace is SA used by plugins like exec and terminal with limited privileges
	cheServiceAccount := deploy.NewServiceAccount(instance, "che")
	if err := r.CreateServiceAccount(instance, cheServiceAccount); err != nil {
		return reconcile.Result{}, err
	}
	workspaceServiceAccount := deploy.NewServiceAccount(instance, "che-workspace")
	if err := r.CreateServiceAccount(instance, workspaceServiceAccount); err != nil {
		return reconcile.Result{}, err
	}
	// create exec and view roles for CheCluster server and workspaces
	execRole := deploy.NewRole(instance, "exec", []string{"pods/exec"}, []string{"*"})
	if err := r.CreateNewRole(instance, execRole); err != nil {
		return reconcile.Result{}, err
	}
	viewRole := deploy.NewRole(instance, "view", []string{"pods"}, []string{"list"})
	if err := r.CreateNewRole(instance, viewRole); err != nil {
		return reconcile.Result{}, err
	}
	// create RoleBindings for created (and existing ClusterRole) roles and service accounts
	cheRoleBinding := deploy.NewRoleBinding(instance, "che", cheServiceAccount.Name, "edit", "ClusterRole")
	if err := r.CreateNewRoleBinding(instance, cheRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	execRoleBinding := deploy.NewRoleBinding(instance, "che-workspace-exec", workspaceServiceAccount.Name, execRole.Name, "Role")
	if err = r.CreateNewRoleBinding(instance, execRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	viewRoleBinding := deploy.NewRoleBinding(instance, "che-workspace-view", workspaceServiceAccount.Name, viewRole.Name, "Role")
	if err := r.CreateNewRoleBinding(instance, viewRoleBinding); err != nil {
		return reconcile.Result{}, err
	}

	// If the user specified an additional cluster role to use for the Che workspace, create a role binding for it
	// Use a role binding instead of a cluster role binding to keep the additional access scoped to the workspace's namespace
	workspaceClusterRole := instance.Spec.Server.CheWorkspaceClusterRole
	if workspaceClusterRole != "" {
		customRoleBinding := deploy.NewRoleBinding(instance, "che-workspace-custom", workspaceServiceAccount.Name, workspaceClusterRole, "ClusterRole")
		if err = r.CreateNewRoleBinding(instance, customRoleBinding); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := r.GenerateAndSaveFields(instance, request); err != nil {
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

		if k8sclient.IsPVCExists(deploy.DefaultPostgresVolumeClaimName, instance.Namespace) {
			k8sclient.DeletePVC(deploy.DefaultPostgresVolumeClaimName, instance.Namespace)
		}
	} else {
		if !tests {
			if k8sclient.IsPVCExists(deploy.DefaultCheVolumeClaimName, instance.Namespace) {
				k8sclient.DeletePVC(deploy.DefaultCheVolumeClaimName, instance.Namespace)
			}
		}
	}

	// Create Postgres resources and provisioning unless an external DB is used
	externalDB := instance.Spec.Database.ExternalDb
	if !externalDB {
		if cheMultiUser == "false" {
			if k8sclient.IsDeploymentExists(deploy.PostgresDeploymentName, instance.Namespace) {
				k8sclient.DeleteDeployment(deploy.PostgresDeploymentName, instance.Namespace)
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
					_, password, err := k8sclient.ReadSecret(identityProviderPostgresSecret, instance.Namespace)
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
					podToExec, err := k8sclient.GetDeploymentPod(deploy.PostgresDeploymentName, instance.Namespace)
					if err != nil {
						return reconcile.Result{}, err
					}
					provisioned := ExecIntoPod(podToExec, pgCommand, "create Keycloak DB, user, privileges", instance.Namespace)
					if provisioned {
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
			if k8sclient.IsDeploymentExists("keycloak", instance.Namespace) {
				k8sclient.DeleteDeployment("keycloak", instance.Namespace)
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
				keycloakRealmClientStatus := instance.Status.KeycloakProvisoned
				if !keycloakRealmClientStatus {
					if err := r.CreateKeycloakResources(instance, request, deploy.KeycloakDeploymentName); err != nil {
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
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

		if err != nil {
			return reconcile.Result{}, err
		}
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
		if err != nil {
			return reconcile.Result{}, err
		}
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
	cheFlavor := instance.Spec.Server.CheFlavor
	cheHost := instance.Spec.Server.CheHost
	preparedConsoleLink := &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploy.DefaultConsoleLinkName,
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: protocol + "://" + cheHost,
				Text: deploy.DefaultConsoleLinkDisplayName(cheFlavor)},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  deploy.DefaultConsoleLinkSection,
				ImageURL: fmt.Sprintf("%s://%s%s", protocol, cheHost, deploy.DefaultConsoleLinkImage),
			},
		},
	}

	existingConsoleLink := &consolev1.ConsoleLink{}

	if getErr := r.nonCachedClient.Get(context.TODO(), client.ObjectKey{Name: deploy.DefaultConsoleLinkName}, existingConsoleLink); getErr == nil {
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

// GetEndpointTlsCrt creates a test TLS route and gets it to extract certificate chain
// There's an easier way which is to read tls secret in default (3.11) or openshift-ingress (4.0) namespace
// which however requires extra privileges for operator service account
func (r *ReconcileChe) GetEndpointTlsCrt(instance *orgv1.CheCluster, endpointUrl string, clusterAPI deploy.ClusterAPI) (certificate []byte, err error) {
	var requestURL string
	var routeStatus deploy.RouteProvisioningStatus

	if len(endpointUrl) < 1 {
		for wait := true; wait; {
			routeStatus = deploy.SyncRouteToCluster(instance, "test", "test", 8080, clusterAPI)
			if routeStatus.Err != nil {
				return nil, routeStatus.Err
			}
			wait = !routeStatus.Continue || len(routeStatus.Route.Spec.Host) == 0
			time.Sleep(time.Duration(1) * time.Second)
		}

		requestURL = "https://" + routeStatus.Route.Spec.Host

	} else {
		requestURL = endpointUrl
	}

	//adding the proxy settings to the Transport object
	transport := &http.Transport{}

	if instance.Spec.Server.ProxyURL != "" {
		logrus.Infof("Configuring proxy with %s to extract crt from the following URL: %s", instance.Spec.Server.ProxyURL, requestURL)
		r.configureProxy(instance, transport)
	}

	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{
		Transport: transport,
	}
	req, err := http.NewRequest("GET", requestURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("An error occurred when reaching test TLS route: %s", err)
		if r.tests {
			fakeCrt := make([]byte, 5)
			return fakeCrt, nil
		}
		return nil, err
	}

	for i := range resp.TLS.PeerCertificates {
		cert := resp.TLS.PeerCertificates[i].Raw
		crt := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		})
		certificate = append(certificate, crt...)
	}

	if routeStatus.Route != nil {
		logrus.Infof("Deleting a test route %s to extract routes crt", routeStatus.Route.Name)
		if err := r.client.Delete(context.TODO(), routeStatus.Route); err != nil {
			logrus.Errorf("Failed to delete test route %s: %s", routeStatus.Route.Name, err)
		}
	}
	return certificate, nil
}

func (r *ReconcileChe) configureProxy(instance *orgv1.CheCluster, transport *http.Transport) {
	proxyParts := strings.Split(instance.Spec.Server.ProxyURL, "://")
	proxyProtocol := ""
	proxyHost := ""
	if len(proxyParts) == 1 {
		proxyProtocol = ""
		proxyHost = proxyParts[0]
	} else {
		proxyProtocol = proxyParts[0]
		proxyHost = proxyParts[1]

	}

	proxyURL := proxyHost
	if instance.Spec.Server.ProxyPort != "" {
		proxyURL = proxyURL + ":" + instance.Spec.Server.ProxyPort
	}

	proxyUser := instance.Spec.Server.ProxyUser
	proxyPassword := instance.Spec.Server.ProxyPassword
	proxySecret := instance.Spec.Server.ProxySecret
	user, password, err := k8sclient.ReadSecret(proxySecret, instance.Namespace)
	if err == nil {
		proxyUser = user
		proxyPassword = password
	} else {
		logrus.Errorf("Failed to read '%s' secret: %s", proxySecret, err)
	}
	if len(proxyUser) > 1 && len(proxyPassword) > 1 {
		proxyURL = proxyUser + ":" + proxyPassword + "@" + proxyURL
	}

	if proxyProtocol != "" {
		proxyURL = proxyProtocol + "://" + proxyURL
	}
	config := httpproxy.Config{
		HTTPProxy:  proxyURL,
		HTTPSProxy: proxyURL,
		NoProxy:    strings.Replace(instance.Spec.Server.NonProxyHosts, "|", ",", -1),
	}
	proxyFunc := config.ProxyFunc()
	transport.Proxy = func(r *http.Request) (*url.URL, error) {
		theProxyUrl, err := proxyFunc(r.URL)
		if err != nil {
			logrus.Warnf("Error when trying to get the proxy to access TLS endpoint URL: %s - %s", r.URL, err)
		}
		logrus.Infof("Using proxy: %s to access TLS endpoint URL: %s", theProxyUrl, r.URL)
		return theProxyUrl, err
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
