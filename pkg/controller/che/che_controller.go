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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
	k8sclient = GetK8Client()
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
	howToCreateAUserLinkOS4 = "https://docs.openshift.com/container-platform/4.1/authentication/understanding-identity-provider.html#identity-provider-overview_understanding-identity-provider"
	howToCreateAUserLinkOS3 = "https://docs.openshift.com/container-platform/3.11/install_config/configuring_authentication.html"
)

// Reconcile reads that state of the cluster for a CheCluster object and makes changes based on the state read
// and what is in the CheCluster.Spec. The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileChe) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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
	if err == nil && !instance.IsAirGapMode() {
		logrus.Info("Found devfile-registry ConfigMap and not in airgap mode.  Deleting.")
		if err = r.client.Delete(context.TODO(), devfileRegistryConfigMap); err != nil {
			logrus.Errorf("Error deleting devfile-registry ConfigMap: %v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

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

	// If the plugin-registry ConfigMap exists, and we are not in airgapped mode, delete the ConfigMap

	if isOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if instance.Spec.Server.SelfSignedCert ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(isOpenShift4 && instance.Spec.Auth.OpenShiftoAuth && !instance.Spec.Server.TlsSupport) {
			if err := r.CreateTLSSecret(instance, "", "self-signed-certificate"); err != nil {
				return reconcile.Result{}, err
			}
		}

		if !tests {
			deployment := &appsv1.Deployment{}
			name := "che"
			cheFlavor := instance.Spec.Server.CheFlavor
			if cheFlavor == "codeready" {
				name = cheFlavor
			}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, deployment)
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
				return reconcile.Result{}, err
			}
			if len(users.Items) < 1 {
				helpLink := ""
				if isOpenShift4 {
					helpLink = howToCreateAUserLinkOS4
				} else {
					helpLink = howToCreateAUserLinkOS3
				}
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
				if err := r.CreateTLSSecret(instance, baseURL, "openshift-api-crt"); err != nil {
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
	chePostgresPassword := instance.Spec.Database.ChePostgresPassword
	keycloakPostgresPassword := instance.Spec.Auth.IdentityProviderPostgresPassword
	keycloakAdminPassword := instance.Spec.Auth.IdentityProviderPassword

	cheFlavor := util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor)
	cheMultiUser := deploy.GetCheMultiUser(instance)

	if cheMultiUser == "false" {
		cheLabels := deploy.GetLabels(instance, cheFlavor)
		pvc := deploy.NewPvc(instance, deploy.DefaultCheVolumeClaimName, "1Gi", cheLabels)
		if err := r.CreatePVC(instance, pvc); err != nil {
			return reconcile.Result{}, err
		}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: instance.Namespace}, pvc)
		if pvc.Status.Phase != "Bound" {
			k8sclient.GetPVCStatus(pvc, instance.Namespace)
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
			if k8sclient.IsDeploymentExists("postgres", instance.Namespace) {
				k8sclient.DeleteDeployment("postgres", instance.Namespace)
			}
		} else {
			// Create a new postgres service
			postgresLabels := deploy.GetLabels(instance, "postgres")
			postgresService := deploy.NewService(instance, "postgres", []string{"postgres"}, []int32{5432}, postgresLabels)
			if err := r.CreateService(instance, postgresService, false); err != nil {
				return reconcile.Result{}, err
			}
			// Create a new Postgres PVC object
			pvc := deploy.NewPvc(instance, deploy.DefaultPostgresVolumeClaimName, "1Gi", postgresLabels)
			if err := r.CreatePVC(instance, pvc); err != nil {
				return reconcile.Result{}, err
			}
			if !tests {
				err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: instance.Namespace}, pvc)
				if pvc.Status.Phase != "Bound" {
					k8sclient.GetPVCStatus(pvc, instance.Namespace)
				}
			}
			// Create a new Postgres deployment
			postgresDeployment := deploy.NewPostgresDeployment(instance, chePostgresPassword, isOpenShift, cheFlavor)

			if err := r.CreateNewDeployment(instance, postgresDeployment); err != nil {
				return reconcile.Result{}, err
			}
			time.Sleep(time.Duration(1) * time.Second)
			pgDeployment, err := r.GetEffectiveDeployment(instance, postgresDeployment.Name)
			if err != nil {
				logrus.Errorf("Failed to get %s deployment: %s", postgresDeployment.Name, err)
				return reconcile.Result{}, err
			}
			if !tests {
				if pgDeployment.Status.AvailableReplicas != 1 {
					scaled := k8sclient.GetDeploymentStatus("postgres", instance.Namespace)
					if !scaled {
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
					}
				}

				desiredImage := util.GetValue(instance.Spec.Database.PostgresImage, deploy.DefaultPostgresImage(instance))
				effectiveImage := pgDeployment.Spec.Template.Spec.Containers[0].Image
				desiredImagePullPolicy := util.GetValue(string(instance.Spec.Database.PostgresImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(desiredImage))
				effectiveImagePullPolicy := string(pgDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
				if effectiveImage != desiredImage ||
					effectiveImagePullPolicy != desiredImagePullPolicy {
					newPostgresDeployment := deploy.NewPostgresDeployment(instance, chePostgresPassword, isOpenShift, cheFlavor)
					logrus.Infof(`Updating Postgres deployment with:
	- Docker Image: %s => %s
	- Image Pull Policy: %s => %s`,
						effectiveImage, desiredImage,
						effectiveImagePullPolicy, desiredImagePullPolicy,
					)
					if err := controllerutil.SetControllerReference(instance, newPostgresDeployment, r.scheme); err != nil {
						logrus.Errorf("An error occurred: %s", err)
					}
					if err := r.client.Update(context.TODO(), newPostgresDeployment); err != nil {
						logrus.Errorf("Failed to update Postgres deployment: %s", err)
					}
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}

				pgCommand := deploy.GetPostgresProvisionCommand(instance)
				dbStatus := instance.Status.DbProvisoned
				// provision Db and users for Che and Keycloak servers
				if !dbStatus {
					podToExec, err := k8sclient.GetDeploymentPod(pgDeployment.Name, instance.Namespace)
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
	cheLabels := deploy.GetLabels(instance, util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor))

	if _, err := deploy.NewCheService(instance, cheLabels, r); err != nil {
		return reconcile.Result{}, err
	}
	if !isOpenShift {
		cheIngress := deploy.NewIngress(instance, cheFlavor, "che-host", 8080)
		if err := r.CreateNewIngress(instance, cheIngress); err != nil {
			return reconcile.Result{}, err
		}
		cheHost := ingressDomain
		if ingressStrategy == "multi-host" {
			cheHost = cheFlavor + "-" + instance.Namespace + "." + ingressDomain
		}
		if len(instance.Spec.Server.CheHost) == 0 {
			instance.Spec.Server.CheHost = cheHost
			if err := r.UpdateCheCRSpec(instance, "CheHost URL", cheHost); err != nil {
				instance, _ = r.GetCR(request)
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
			}
		}
	} else {
		cheRoute := deploy.NewRoute(instance, cheFlavor, "che-host", 8080)
		if tlsSupport {
			cheRoute = deploy.NewTlsRoute(instance, cheFlavor, "che-host", 8080)
		}
		if err := r.CreateNewRoute(instance, cheRoute); err != nil {
			return reconcile.Result{}, err
		}
		if len(instance.Spec.Server.CheHost) == 0 {
			instance.Spec.Server.CheHost = cheRoute.Spec.Host
			if len(cheRoute.Spec.Host) < 1 {
				cheRoute := r.GetEffectiveRoute(instance, cheRoute.Name)
				instance.Spec.Server.CheHost = cheRoute.Spec.Host
			}
			if err := r.UpdateCheCRSpec(instance, "CheHost URL", instance.Spec.Server.CheHost); err != nil {
				instance, _ = r.GetCR(request)
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
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
			keycloakService := deploy.NewService(instance, "keycloak", []string{"http"}, []int32{8080}, keycloakLabels)
			if err := r.CreateService(instance, keycloakService, false); err != nil {
				return reconcile.Result{}, err
			}
			// create Keycloak ingresses when on k8s
			if !isOpenShift {
				keycloakIngress := deploy.NewIngress(instance, "keycloak", "keycloak", 8080)
				if err := r.CreateNewIngress(instance, keycloakIngress); err != nil {
					return reconcile.Result{}, err
				}
				keycloakURL := protocol + "://" + ingressDomain
				if ingressStrategy == "multi-host" {
					keycloakURL = protocol + "://keycloak-" + instance.Namespace + "." + ingressDomain
				}
				if len(instance.Spec.Auth.IdentityProviderURL) == 0 {
					instance.Spec.Auth.IdentityProviderURL = keycloakURL
					if err := r.UpdateCheCRSpec(instance, "Keycloak URL", instance.Spec.Auth.IdentityProviderURL); err != nil {
						instance, _ = r.GetCR(request)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
					}
				}
			} else {
				// create Keycloak route
				keycloakRoute := deploy.NewRoute(instance, "keycloak", "keycloak", 8080)
				if tlsSupport {
					keycloakRoute = deploy.NewTlsRoute(instance, "keycloak", "keycloak", 8080)
				}
				if err = r.CreateNewRoute(instance, keycloakRoute); err != nil {
					return reconcile.Result{}, err
				}
				keycloakURL := keycloakRoute.Spec.Host
				if len(instance.Spec.Auth.IdentityProviderURL) == 0 {
					instance.Spec.Auth.IdentityProviderURL = protocol + "://" + keycloakURL
					if len(keycloakURL) < 1 {
						keycloakURL := r.GetEffectiveRoute(instance, keycloakRoute.Name).Spec.Host
						instance.Spec.Auth.IdentityProviderURL = protocol + "://" + keycloakURL
					}
					if err := r.UpdateCheCRSpec(instance, "Keycloak URL", instance.Spec.Auth.IdentityProviderURL); err != nil {
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
					}
					instance.Status.KeycloakURL = protocol + "://" + keycloakURL
					if err := r.UpdateCheCRStatus(instance, "status: Keycloak URL", instance.Spec.Auth.IdentityProviderURL); err != nil {
						instance, _ = r.GetCR(request)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
					}
				}
			}
			keycloakDeployment := deploy.NewKeycloakDeployment(instance, keycloakPostgresPassword, keycloakAdminPassword, cheFlavor,
				r.GetEffectiveSecretResourceVersion(instance, "self-signed-certificate"),
				r.GetEffectiveSecretResourceVersion(instance, "openshift-api-crt"))
			if err := r.CreateNewDeployment(instance, keycloakDeployment); err != nil {
				return reconcile.Result{}, err
			}
			time.Sleep(time.Duration(1) * time.Second)
			effectiveKeycloakDeployment, err := r.GetEffectiveDeployment(instance, keycloakDeployment.Name)
			if err != nil {
				logrus.Errorf("Failed to get %s deployment: %s", keycloakDeployment.Name, err)
				return reconcile.Result{}, err
			}
			if !tests {
				if effectiveKeycloakDeployment.Status.AvailableReplicas != 1 {
					scaled := k8sclient.GetDeploymentStatus(keycloakDeployment.Name, instance.Namespace)
					if !scaled {
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
					}
				}

				if effectiveKeycloakDeployment.Status.Replicas > 1 {
					logrus.Infof("Deployment %s is in the rolling update state", "keycloak")
					k8sclient.GetDeploymentRollingUpdateStatus("keycloak", instance.Namespace)
				}

				desiredImage := util.GetValue(instance.Spec.Auth.IdentityProviderImage, deploy.DefaultKeycloakImage(instance))
				effectiveImage := effectiveKeycloakDeployment.Spec.Template.Spec.Containers[0].Image
				desiredImagePullPolicy := util.GetValue(string(instance.Spec.Auth.IdentityProviderImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(desiredImage))
				effectiveImagePullPolicy := string(effectiveKeycloakDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
				cheCertSecretVersion := r.GetEffectiveSecretResourceVersion(instance, "self-signed-certificate")
				storedCheCertSecretVersion := effectiveKeycloakDeployment.Annotations["che.self-signed-certificate.version"]
				openshiftApiCertSecretVersion := r.GetEffectiveSecretResourceVersion(instance, "openshift-api-crt")
				storedOpenshiftApiCertSecretVersion := effectiveKeycloakDeployment.Annotations["che.openshift-api-crt.version"]
				if effectiveImage != desiredImage ||
					effectiveImagePullPolicy != desiredImagePullPolicy ||
					cheCertSecretVersion != storedCheCertSecretVersion ||
					openshiftApiCertSecretVersion != storedOpenshiftApiCertSecretVersion {
					newKeycloakDeployment := deploy.NewKeycloakDeployment(instance, keycloakPostgresPassword, keycloakAdminPassword, cheFlavor, cheCertSecretVersion, openshiftApiCertSecretVersion)
					logrus.Infof(`Updating Keycloak deployment with:
	- Docker Image: %s => %s
	- Image Pull Policy: %s => %s
	- Self-Signed Certificate Version: %s => %s
	- OpenShift API Certificate Version: %s => %s`,
						effectiveImage, desiredImage,
						effectiveImagePullPolicy, desiredImagePullPolicy,
						cheCertSecretVersion, storedCheCertSecretVersion,
						openshiftApiCertSecretVersion, storedOpenshiftApiCertSecretVersion,
					)
					if err := controllerutil.SetControllerReference(instance, newKeycloakDeployment, r.scheme); err != nil {
						logrus.Errorf("An error occurred: %s", err)
					}
					if err := r.client.Update(context.TODO(), newKeycloakDeployment); err != nil {
						logrus.Errorf("Failed to update Keycloak deployment: %s", err)
					}
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}
				keycloakRealmClientStatus := instance.Status.KeycloakProvisoned
				if !keycloakRealmClientStatus {
					if err := r.CreateKeycloakResources(instance, request, keycloakDeployment.Name); err != nil {
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
					}
				}
			}

			if isOpenShift {
				doInstallOpenShiftoAuthProvider := instance.Spec.Auth.OpenShiftoAuth
				if doInstallOpenShiftoAuthProvider {
					openShiftIdentityProviderStatus := instance.Status.OpenShiftoAuthProvisioned
					if !openShiftIdentityProviderStatus {
						if err := r.CreateIdentityProviderItems(instance, request, cheFlavor, keycloakDeployment.Name, isOpenShift4); err != nil {
							return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
						}
					}
				}
			}
		}
	}

	addRegistryRoute := func(registryType string) (string, error) {
		registryName := registryType + "-registry"
		host := ""
		if !isOpenShift {
			ingress := deploy.NewIngress(instance, registryName, registryName, 8080)
			if err := r.CreateNewIngress(instance, ingress); err != nil {
				return "", err
			}
			host = ingressDomain
			if ingressStrategy == "multi-host" {
				host = registryName + "-" + instance.Namespace + "." + ingressDomain
			}
		} else {
			route := deploy.NewRoute(instance, registryName, registryName, 8080)
			if tlsSupport {
				route = deploy.NewTlsRoute(instance, registryName, registryName, 8080)
			}
			if err := r.CreateNewRoute(instance, route); err != nil {
				return "", err
			}
			host = route.Spec.Host
			if len(host) < 1 {
				cheRoute := r.GetEffectiveRoute(instance, route.Name)
				host = cheRoute.Spec.Host
			}
		}
		return protocol + "://" + host, nil
	}

	addRegistryDeployment := func(
		registryType string,
		registryImage string,
		registryImagePullPolicy corev1.PullPolicy,
		registryMemoryLimit string,
		registryMemoryRequest string,
		probePath string,
	) (*reconcile.Result, error) {
		registryName := registryType + "-registry"

		// Create a new registry service
		registryLabels := deploy.GetLabels(instance, registryName)
		registryService := deploy.NewService(instance, registryName, []string{"http"}, []int32{8080}, registryLabels)
		if err := r.CreateService(instance, registryService, true); err != nil {
			return &reconcile.Result{}, err
		}
		// Create a new registry deployment
		registryDeployment := deploy.NewRegistryDeployment(
			instance,
			registryType,
			registryImage,
			registryImagePullPolicy,
			registryMemoryLimit,
			registryMemoryRequest,
			probePath,
		)
		if err := r.CreateNewDeployment(instance, registryDeployment); err != nil {
			return &reconcile.Result{}, err
		}
		time.Sleep(time.Duration(1) * time.Second)
		effectiveDeployment, err := r.GetEffectiveDeployment(instance, registryDeployment.Name)
		if err != nil {
			logrus.Errorf("Failed to get %s deployment: %s", registryDeployment.Name, err)
			return &reconcile.Result{}, err
		}
		if !tests {
			if effectiveDeployment.Status.AvailableReplicas != 1 {
				scaled := k8sclient.GetDeploymentStatus(registryName, instance.Namespace)
				if !scaled {
					return &reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}
			}

			if effectiveDeployment.Status.Replicas > 1 {
				logrus.Infof("Deployment %s is in the rolling update state", registryName)
				k8sclient.GetDeploymentRollingUpdateStatus(registryName, instance.Namespace)
			}

			desiredMemRequest, err := resource.ParseQuantity(registryMemoryRequest)
			if err != nil {
				logrus.Errorf("Wrong quantity for %s deployment Memory Request: %s", registryName, err)
				return &reconcile.Result{}, err
			}
			effectiveMemRequest := effectiveDeployment.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
			desiredMemLimit, err := resource.ParseQuantity(registryMemoryLimit)
			if err != nil {
				logrus.Errorf("Wrong quantity for %s deployment Memory Limit: %s", registryName, err)
				return &reconcile.Result{}, err
			}
			effectiveMemLimit := effectiveDeployment.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]
			effectiveRegistryImage := effectiveDeployment.Spec.Template.Spec.Containers[0].Image
			effectiveRegistryImagePullPolicy := effectiveDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy
			if effectiveRegistryImage != registryImage ||
				effectiveMemRequest.Cmp(desiredMemRequest) != 0 ||
				effectiveMemLimit.Cmp(desiredMemLimit) != 0 ||
				effectiveRegistryImagePullPolicy != registryImagePullPolicy {
				newDeployment := deploy.NewRegistryDeployment(
					instance,
					registryType,
					registryImage,
					registryImagePullPolicy,
					registryMemoryLimit,
					registryMemoryRequest,
					probePath,
				)
				logrus.Infof(`Updating %s registry deployment with:
	- Docker Image: %s => %s
	- Image Pull Policy: %s => %s
	- Memory Request: %s => %s
	- Memory Limit: %s => %s`, registryType,
					effectiveRegistryImage, registryImage,
					effectiveRegistryImagePullPolicy, registryImagePullPolicy,
					effectiveMemRequest.String(), desiredMemRequest.String(),
					effectiveMemLimit.String(), desiredMemLimit.String(),
				)
				if err := controllerutil.SetControllerReference(instance, newDeployment, r.scheme); err != nil {
					logrus.Errorf("An error occurred: %s", err)
				}
				if err := r.client.Update(context.TODO(), newDeployment); err != nil {
					logrus.Errorf("Failed to update %s registry deployment: %s", registryType, err)
				}
				return &reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
			}
		}
		return nil, nil
	}

	devfileRegistryURL := instance.Spec.Server.DevfileRegistryUrl

	// Create devfile registry resources unless an external registry is used
	externalDevfileRegistry := instance.Spec.Server.ExternalDevfileRegistry
	if !externalDevfileRegistry {
		guessedDevfileRegistryURL, err := addRegistryRoute("devfile")
		if err != nil {
			return reconcile.Result{}, err
		}
		if devfileRegistryURL == "" {
			devfileRegistryURL = guessedDevfileRegistryURL
		}
		if instance.IsAirGapMode() {
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
				devFileRegistryConfigMap = deploy.CreateDevfileRegistryConfigMap(instance, devfileRegistryURL)
				err = controllerutil.SetControllerReference(instance, devFileRegistryConfigMap, r.scheme)
				if err != nil {
					logrus.Errorf("An error occurred: %v", err)
					return reconcile.Result{}, err
				}
				logrus.Info("Updating devfile-registry ConfigMap")
				err = r.client.Update(context.TODO(), devFileRegistryConfigMap)
				if err != nil {
					logrus.Errorf("Error updating devfile-registry ConfigMap: %v", err)
					return reconcile.Result{}, err
				}
			}
		}

		devfileRegistryImage := util.GetValue(instance.Spec.Server.DevfileRegistryImage, deploy.DefaultDevfileRegistryImage(instance))
		result, err := addRegistryDeployment(
			"devfile",
			devfileRegistryImage,
			corev1.PullPolicy(util.GetValue(string(instance.Spec.Server.PluginRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(devfileRegistryImage))),
			util.GetValue(string(instance.Spec.Server.DevfileRegistryMemoryLimit), deploy.DefaultDevfileRegistryMemoryLimit),
			util.GetValue(string(instance.Spec.Server.DevfileRegistryMemoryRequest), deploy.DefaultDevfileRegistryMemoryRequest),
			"/devfiles/",
		)
		if err != nil || result != nil {
			return *result, err
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
		guessedPluginRegistryURL, err := addRegistryRoute("plugin")
		if err != nil {
			return reconcile.Result{}, err
		}
		guessedPluginRegistryURL += "/v3"
		if pluginRegistryURL == "" {
			pluginRegistryURL = guessedPluginRegistryURL
		}

		pluginRegistryImage := util.GetValue(instance.Spec.Server.PluginRegistryImage, deploy.DefaultPluginRegistryImage(instance))
		result, err := addRegistryDeployment(
			"plugin",
			pluginRegistryImage,
			corev1.PullPolicy(util.GetValue(string(instance.Spec.Server.PluginRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(pluginRegistryImage))),
			util.GetValue(string(instance.Spec.Server.PluginRegistryMemoryLimit), deploy.DefaultPluginRegistryMemoryLimit),
			util.GetValue(string(instance.Spec.Server.PluginRegistryMemoryRequest), deploy.DefaultPluginRegistryMemoryRequest),
			"/v3/plugins/",
		)
		if err != nil || result != nil {
			return *result, err
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
	cheHost := instance.Spec.Server.CheHost
	cheEnv := deploy.GetConfigMapData(instance)
	cheConfigMap := deploy.NewCheConfigMap(instance, cheEnv)
	if err := r.CreateNewConfigMap(instance, cheConfigMap); err != nil {
		return reconcile.Result{}, err
	}

	// configMap resource version will be an env in Che deployment to easily update it when a ConfigMap changes
	// which will automatically trigger Che rolling update
	cmResourceVersion := cheConfigMap.ResourceVersion

	cheImageAndTag := GetFullCheServerImageLink(instance)
	cheVersion := EvaluateCheServerVersion(instance)

	// create Che deployment
	cheDeploymentToCreate, err := deploy.NewCheDeployment(instance, cheImageAndTag, cmResourceVersion, isOpenShift)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err = r.CreateNewDeployment(instance, cheDeploymentToCreate); err != nil {
		return reconcile.Result{}, err
	}
	// sometimes Get cannot find deployment right away
	time.Sleep(time.Duration(1) * time.Second)
	effectiveCheDeployment, err := r.GetEffectiveDeployment(instance, cheDeploymentToCreate.Name)
	if err != nil {
		logrus.Errorf("Failed to get %s deployment: %s", cheDeploymentToCreate.Name, err)
		return reconcile.Result{}, err
	}
	if !tests {
		if effectiveCheDeployment.Status.Replicas > 1 {
			// Specific case: a Rolling update is happening
			logrus.Infof("Deployment %s is in the rolling update state", cheDeploymentToCreate.Name)
			if err := r.SetCheRollingUpdateStatus(instance, request); err != nil {
				instance, _ = r.GetCR(request)
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
			}
			k8sclient.GetDeploymentRollingUpdateStatus(cheDeploymentToCreate.Name, instance.Namespace)
			deployment, _ := r.GetEffectiveDeployment(instance, cheDeploymentToCreate.Name)
			if deployment.Status.Replicas == 1 {
				if err := r.SetCheAvailableStatus(instance, request, protocol, cheHost); err != nil {
					instance, _ = r.GetCR(request)
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
			}
		} else {
			if effectiveCheDeployment.Status.AvailableReplicas < 1 {
				// Deployment was just created
				instance, _ := r.GetCR(request)
				if err := r.SetCheUnavailableStatus(instance, request); err != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
				scaled := k8sclient.GetDeploymentStatus(cheDeploymentToCreate.Name, instance.Namespace)
				if !scaled {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}
				effectiveCheDeployment, err = r.GetEffectiveDeployment(instance, cheDeploymentToCreate.Name)
			}
			if effectiveCheDeployment.Status.AvailableReplicas == 1 &&
				instance.Status.CheClusterRunning != AvailableStatus {
				if err := r.SetCheAvailableStatus(instance, request, protocol, cheHost); err != nil {
					instance, _ = r.GetCR(request)
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
				if instance.Status.CheVersion != cheVersion {
					instance.Status.CheVersion = cheVersion
					if err := r.UpdateCheCRStatus(instance, "version", cheVersion); err != nil {
						instance, _ = r.GetCR(request)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
					}
				}
			}
		}
	}

	// we can now try to create consolelink, after che instance is available
	if err := createConsoleLink(isOpenShift4, protocol, instance, r); err != nil {
		logrus.Errorf("An error occurred during console link provisioning: %s", err)
		return reconcile.Result{}, err
	}

	if effectiveCheDeployment.Spec.Template.Spec.Containers[0].Image != cheDeploymentToCreate.Spec.Template.Spec.Containers[0].Image {
		if err := controllerutil.SetControllerReference(instance, cheDeploymentToCreate, r.scheme); err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		logrus.Infof("Updating %s %s with image %s", cheDeploymentToCreate.Name, cheDeploymentToCreate.Kind, cheImageAndTag)
		instance.Status.CheVersion = cheVersion
		if err := r.UpdateCheCRStatus(instance, "version", cheVersion); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
		if err := r.client.Update(context.TODO(), cheDeploymentToCreate); err != nil {
			logrus.Errorf("Failed to update %s %s: %s", effectiveCheDeployment.Kind, effectiveCheDeployment.Name, err)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err

		}
	}

	// reconcile routes/ingresses before reconciling Che deployment
	activeConfigMap := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: instance.Namespace}, activeConfigMap); err != nil {
		logrus.Errorf("ConfigMap %s not found: %s", activeConfigMap.Name, err)
	}
	if !tlsSupport && activeConfigMap.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"] == "true" {
		routesUpdated, err := r.ReconcileTLSObjects(instance, request, cheFlavor, tlsSupport, isOpenShift)
		if err != nil {
			logrus.Errorf("An error occurred when updating routes %s", err)
		}
		if routesUpdated {
			logrus.Info("Routes have been updated with TLS config")
		}
	}
	if tlsSupport && activeConfigMap.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"] == "false" {
		routesUpdated, err := r.ReconcileTLSObjects(instance, request, cheFlavor, tlsSupport, isOpenShift)
		if err != nil {
			logrus.Errorf("An error occurred when updating routes %s", err)
		}
		if routesUpdated {
			logrus.Info("Routes have been updated with TLS config")
		}
	}
	// Reconcile Che ConfigMap to align with CR spec
	cmUpdated, err := r.UpdateConfigMap(instance)
	if err != nil {
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

	if cmUpdated {
		// sometimes an old cm resource version is returned ie get happens too fast - before server updates CM
		time.Sleep(time.Duration(1) * time.Second)
		cm := r.GetEffectiveConfigMap(instance, cheConfigMap.Name)
		cmResourceVersion := cm.ResourceVersion
		cheDeployment, err := deploy.NewCheDeployment(instance, cheImageAndTag, cmResourceVersion, isOpenShift)
		if err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		if err := controllerutil.SetControllerReference(instance, cheDeployment, r.scheme); err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		if err := r.client.Update(context.TODO(), cheDeployment); err != nil {
			return reconcile.Result{}, err
		}
	}
	effectiveCheDeployment, _ = r.GetEffectiveDeployment(instance, cheDeploymentToCreate.Name)
	effectiveMemRequest := effectiveCheDeployment.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
	effectiveMemLimit := effectiveCheDeployment.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]
	desiredMemRequest, err := resource.ParseQuantity(util.GetValue(instance.Spec.Server.ServerMemoryRequest, deploy.DefaultServerMemoryRequest))
	if err != nil {
		logrus.Errorf("Wrong quantity for Che deployment Memory Request: %s", err)
		return reconcile.Result{}, err
	}
	desiredMemLimit, err := resource.ParseQuantity(util.GetValue(instance.Spec.Server.ServerMemoryLimit, deploy.DefaultServerMemoryLimit))
	if err != nil {
		logrus.Errorf("Wrong quantity for Che deployment Memory Limit: %s", err)
		return reconcile.Result{}, err
	}
	desiredImagePullPolicy := util.GetValue(string(instance.Spec.Server.CheImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(cheImageAndTag))
	effectiveImagePullPolicy := string(effectiveCheDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
	desiredSelfSignedCert := instance.Spec.Server.SelfSignedCert
	desiredCustomPublicCerts := instance.Spec.Server.ServerTrustStoreConfigMapName != ""
	desiredGitSelfSignedCert := instance.Spec.Server.GitSelfSignedCert
	effectiveSelfSignedCert := util.GetDeploymentEnvVarSource(effectiveCheDeployment, "CHE_SELF__SIGNED__CERT") != nil
	effectiveCustomPublicCerts := r.GetDeploymentVolume(effectiveCheDeployment, "che-public-certs").ConfigMap != nil
	effectiveGitSelfSignedCert := util.GetDeploymentEnvVarSource(effectiveCheDeployment, "CHE_GIT_SELF__SIGNED__CERT") != nil
	if desiredMemRequest.Cmp(effectiveMemRequest) != 0 ||
		desiredMemLimit.Cmp(effectiveMemLimit) != 0 ||
		effectiveImagePullPolicy != desiredImagePullPolicy ||
		effectiveSelfSignedCert != desiredSelfSignedCert ||
		effectiveCustomPublicCerts != desiredCustomPublicCerts ||
		effectiveGitSelfSignedCert != desiredGitSelfSignedCert {
		cheDeployment, err := deploy.NewCheDeployment(instance, cheImageAndTag, cmResourceVersion, isOpenShift)
		if err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		if err := controllerutil.SetControllerReference(instance, cheDeployment, r.scheme); err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		logrus.Infof(`Updating deployment %s with:
	- Memory Request: %s => %s
	- Memory Limit: %s => %s
	- Image Pull Policy: %s => %s
	- Self-Signed Cert: %t => %t`,
			cheDeployment.Name,
			effectiveMemRequest.String(), desiredMemRequest.String(),
			effectiveMemLimit.String(), desiredMemLimit.String(),
			effectiveImagePullPolicy, desiredImagePullPolicy,
			effectiveSelfSignedCert, desiredSelfSignedCert,
		)
		if err := r.client.Update(context.TODO(), cheDeployment); err != nil {
			logrus.Errorf("Failed to update deployment: %s", err)
			return reconcile.Result{}, err
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

// GetFullCheServerImageLink evaluate full cheImage link(with repo and tag)
// based on Checluster information and image defaults from env variables
func GetFullCheServerImageLink(cr *orgv1.CheCluster) string {
	if len(cr.Spec.Server.CheImage) > 0 {
		cheServerImageTag := util.GetValue(cr.Spec.Server.CheImageTag, deploy.DefaultCheVersion())
		return cr.Spec.Server.CheImage + ":" + cheServerImageTag
	}

	defaultCheServerImage := deploy.DefaultCheServerImage(cr)
	if len(cr.Spec.Server.CheImageTag) == 0 {
		return defaultCheServerImage
	}

	// For back compatibility with version < 7.9.0:
	// if cr.Spec.Server.CheImage is empty, but cr.Spec.Server.CheImageTag is not empty,
	// parse from default Che image(value comes from env variable) "Che image repository"
	// and return "Che image", like concatenation: "cheImageRepo:cheImageTag"
	separator := map[bool]string{true: "@", false: ":"}[strings.Contains(defaultCheServerImage, "@")]
	imageParts := strings.Split(defaultCheServerImage, separator)
	return imageParts[0] + ":" + cr.Spec.Server.CheImageTag
}

// EvaluateCheServerVersion evaluate che version
// based on Checluster information and image defaults from env variables
func EvaluateCheServerVersion(cr *orgv1.CheCluster) string {
	return util.GetValue(cr.Spec.Server.CheImageTag, deploy.DefaultCheVersion())
}
