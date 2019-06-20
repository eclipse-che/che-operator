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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
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
	"time"
)

var log = logf.Log.WithName("controller_che")
var (
	k8sclient = GetK8Client()
)

// Add creates a new CheCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileChe{client: mgr.GetClient(), scheme: mgr.GetScheme()}
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
	// register OpenShift routes in the scheme
	if isOpenShift {
		if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift route to scheme: %s", err)
		}
		if err := oauth.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add oAuth to scheme: %s", err)
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
	scheme *runtime.Scheme
	tests  bool
}

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
		doInstallOpenShiftoAuthProvider := instance.Spec.Auth.OpenShiftOauth
		if doInstallOpenShiftoAuthProvider {
			if err := r.ReconcileFinalizer(instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	if isOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if instance.Spec.Server.SelfSignedCert ||
			// To use Openshift v4 OAuth, the OAuth endpoints are served from a namespace
			// and NOT from the Openshift API Master URL (as in v3)
			// So we also need the self-signed certificate to access them (same as the Che server)
			(isOpenShift4 && instance.Spec.Auth.OpenShiftOauth && ! instance.Spec.Server.TlsSupport) {
			if err := r.CreateTLSSecret(instance, "", "self-signed-certificate"); err != nil {
				return reconcile.Result{}, err
			}
		}
		if instance.Spec.Auth.OpenShiftOauth {
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
	keycloakPostgresPassword := instance.Spec.Auth.KeycloakPostgresPassword
	keycloakAdminPassword := instance.Spec.Auth.KeycloakAdminPassword

	// Create Postgres resources and provisioning unless an external DB is used
	externalDB := instance.Spec.Database.ExternalDB
	if !externalDB {
		// Create a new postgres service
		postgresLabels := deploy.GetLabels(instance, "postgres")
		postgresService := deploy.NewService(instance, "postgres", []string{"postgres"}, []int32{5432}, postgresLabels)
		if err := r.CreateService(instance, postgresService); err != nil {
			return reconcile.Result{}, err
		}
		// Create a new Postgres PVC object
		pvc := deploy.NewPvc(instance, "postgres-data", "1Gi", postgresLabels)
		if err := r.CreatePVC(instance, pvc); err != nil {
			return reconcile.Result{}, err
		}
		if !tests {
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: instance.Namespace}, pvc)
			if pvc.Status.Phase != "Bound" {
				k8sclient.GetPostgresStatus(pvc, instance.Namespace)
			}
		}
		// Create a new Postgres deployment
		postgresDeployment := deploy.NewPostgresDeployment(instance, chePostgresPassword, isOpenShift)
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
						if err := r.UpdateCheCRStatus(instance, "status: provisioned with DB and user", "true"); err != nil {
							instance, _ = r.GetCR(request)
						} else {
							break
						}
					}
				} else {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}
			}
		}
	}
	cheFlavor := util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor)
	ingressStrategy := util.GetValue(instance.Spec.K8SOnly.IngressStrategy, deploy.DefaultIngressStrategy)
	ingressDomain := instance.Spec.K8SOnly.IngressDomain
	tlsSupport := instance.Spec.Server.TlsSupport
	protocol := "http"
	if tlsSupport {
		protocol = "https"
	}
	// create Che service and route
	cheLabels := deploy.GetLabels(instance, util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor))

	cheService := deploy.NewService(instance, "che-host", []string{"http", "metrics"}, []int32{8080, 8087}, cheLabels)
	if err := r.CreateService(instance, cheService); err != nil {
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
	ExternalKeycloak := instance.Spec.Auth.ExternalKeycloak

	if !ExternalKeycloak {
		keycloakLabels := deploy.GetLabels(instance, "keycloak")
		keycloakService := deploy.NewService(instance, "keycloak", []string{"http"}, []int32{8080}, keycloakLabels)
		if err := r.CreateService(instance, keycloakService); err != nil {
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
			if len(instance.Spec.Auth.KeycloakURL) == 0 {
				instance.Spec.Auth.KeycloakURL = keycloakURL
				if err := r.UpdateCheCRSpec(instance, "Keycloak URL", instance.Spec.Auth.KeycloakURL); err != nil {
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
			if len(instance.Spec.Auth.KeycloakURL) == 0 {
				instance.Spec.Auth.KeycloakURL = protocol + "://" + keycloakURL
				if len(keycloakURL) < 1 {
					keycloakURL := r.GetEffectiveRoute(instance, keycloakRoute.Name).Spec.Host
					instance.Spec.Auth.KeycloakURL = protocol + "://" + keycloakURL
				}
				if err := r.UpdateCheCRSpec(instance, "Keycloak URL", instance.Spec.Auth.KeycloakURL); err != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
				instance.Status.KeycloakURL = protocol + "://" + keycloakURL
				if err := r.UpdateCheCRStatus(instance, "status: Keycloak URL", instance.Spec.Auth.KeycloakURL); err != nil {
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
		deployment, err := r.GetEffectiveDeployment(instance, keycloakDeployment.Name)
		if err != nil {
			logrus.Errorf("Failed to get %s deployment: %s", keycloakDeployment.Name, err)
			return reconcile.Result{}, err
		}
		if !tests {
			if deployment.Status.AvailableReplicas != 1 {
				scaled := k8sclient.GetDeploymentStatus(keycloakDeployment.Name, instance.Namespace)
				if !scaled {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}
			}

			cheCertSecretVersion := r.GetEffectiveSecretResourceVersion(instance, "self-signed-certificate")
			openshiftApiCertSecretVersion := r.GetEffectiveSecretResourceVersion(instance, "openshift-api-crt")
			if deployment.Spec.Template.Spec.Containers[0].Image != instance.Spec.Auth.KeycloakImage ||
			cheCertSecretVersion != deployment.Annotations["che.self-signed-certificate.version"] ||
			openshiftApiCertSecretVersion != deployment.Annotations["che.openshift-api-crt.version"] {
				keycloakDeployment := deploy.NewKeycloakDeployment(instance, keycloakPostgresPassword, keycloakAdminPassword, cheFlavor, cheCertSecretVersion, openshiftApiCertSecretVersion)
				logrus.Infof("Updating Keycloak deployment with an image %s", instance.Spec.Auth.KeycloakImage)
				if err := controllerutil.SetControllerReference(instance, keycloakDeployment, r.scheme); err != nil {
					logrus.Errorf("An error occurred: %s", err)
				}
				if err := r.client.Update(context.TODO(), keycloakDeployment); err != nil {
					logrus.Errorf("Failed to update Keycloak deployment: %s", err)
				}

			}
			keycloakRealmClientStatus := instance.Status.KeycloakProvisoned
			if !keycloakRealmClientStatus {
				if err := r.CreateKyecloakResources(instance, request, keycloakDeployment.Name); err != nil {
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
				}
			}
		}

		if isOpenShift {
			doInstallOpenShiftoAuthProvider := instance.Spec.Auth.OpenShiftOauth
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
	// create Che ConfigMap which is synced with CR and is not supposed to be manually edited
	// controller will reconcile this CM with CR spec
	cheHost := instance.Spec.Server.CheHost
	cheEnv := deploy.GetConfigMapData(instance)
	cheConfigMap := deploy.NewCheConfigMap(instance, cheEnv)
	if err := r.CreateNewConfigMap(instance, cheConfigMap); err != nil {
		return reconcile.Result{}, err
	}

	// create a custom ConfigMap that won't be synced with CR spec
	// to be able to override envs and not clutter CR spec with fields which are too numerous
	customCM := &corev1.ConfigMap{
		Data: deploy.GetCustomConfigMapData(),
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom",
			Namespace: instance.Namespace,
			Labels:    cheLabels}}
	if err := r.CreateNewConfigMap(instance, customCM); err != nil {
		return reconcile.Result{}, err
	}
	// configMap resource version will be an env in Che deployment to easily update it when a ConfigMap changes
	// which will automatically trigger Che rolling update
	cmResourceVersion := cheConfigMap.ResourceVersion
	// create Che deployment
	cheImageRepo := util.GetValue(instance.Spec.Server.CheImage, deploy.DefaultCheServerImageRepo)
	cheImageTag := util.GetValue(instance.Spec.Server.CheImageTag, deploy.DefaultCheServerImageTag)
	if cheFlavor == "codeready" {
		cheImageRepo = util.GetValue(instance.Spec.Server.CheImage, deploy.DefaultCodeReadyServerImageRepo)
		cheImageTag = util.GetValue(instance.Spec.Server.CheImageTag, deploy.DefaultCodeReadyServerImageTag)
	}
	cheDeployment, err := deploy.NewCheDeployment(instance, cheImageRepo, cheImageTag, cmResourceVersion, isOpenShift)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err = r.CreateNewDeployment(instance, cheDeployment); err != nil {
		return reconcile.Result{}, err
	}
	// sometimes Get cannot find deployment right away
	time.Sleep(time.Duration(1) * time.Second)
	deployment, err := r.GetEffectiveDeployment(instance, cheDeployment.Name)
	if err != nil {
		logrus.Errorf("Failed to get %s deployment: %s", cheDeployment.Name, err)
		return reconcile.Result{}, err
	}
	if !tests {
		if deployment.Status.AvailableReplicas != 1 {
			instance, _ := r.GetCR(request)
			if err := r.SetCheUnavailableStatus(instance, request); err != nil {
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
			}
			scaled := k8sclient.GetDeploymentStatus(cheDeployment.Name, instance.Namespace)
			if !scaled {
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
			}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: cheDeployment.Name, Namespace: instance.Namespace}, deployment)
			if deployment.Status.AvailableReplicas == 1 {
				if err := r.SetCheAvailableStatus(instance, request, protocol, cheHost); err != nil {
					instance, _ = r.GetCR(request)
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
				if instance.Status.CheVersion != cheImageTag {
					instance.Status.CheVersion = cheImageTag
					if err := r.UpdateCheCRStatus(instance, "version", cheImageTag); err != nil {
						instance, _ = r.GetCR(request)
						return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
					}
				}
			}
		}
		if deployment.Status.Replicas > 1 {
			logrus.Infof("Deployment %s is in the rolling update state", cheDeployment.Name)
			if err := r.SetCheRollingUpdateStatus(instance, request); err != nil {
				instance, _ = r.GetCR(request)
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
			}
			k8sclient.GetDeploymentRollingUpdateStatus(cheDeployment.Name, instance.Namespace)
			deployment, _ := r.GetEffectiveDeployment(instance, cheDeployment.Name)
			if deployment.Status.Replicas == 1 {
				if err := r.SetCheAvailableStatus(instance, request, protocol, cheHost); err != nil {
					instance, _ = r.GetCR(request)
					return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
				}
			}
		}
	}
	if deployment.Spec.Template.Spec.Containers[0].Image != cheDeployment.Spec.Template.Spec.Containers[0].Image {
		if err := controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		logrus.Infof("Updating %s %s with image %s:%s", cheDeployment.Name, cheDeployment.Kind, cheImageRepo, cheImageTag)
		instance.Status.CheVersion = cheImageTag
		if err := r.UpdateCheCRStatus(instance, "version", cheImageTag); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
		if err := r.client.Update(context.TODO(), cheDeployment); err != nil {
			logrus.Errorf("Failed to update %s %s: %s", deployment.Kind, deployment.Name, err)
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
		if err := r.DeleteFinalizer(instance); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
		instance.Status.OpenShiftoAuthProvisioned = false
		if err := r.UpdateCheCRStatus(instance, "provisioned with OpenShift oAuth", "false"); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
		instance.Spec.Auth.OauthSecret = ""
		if err := r.UpdateCheCRSpec(instance, "delete oAuth secret name", ""); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
		instance.Spec.Auth.OauthClientName = ""
		if err := r.UpdateCheCRSpec(instance, "delete oAuth client name", ""); err != nil {
			instance, _ = r.GetCR(request)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, err
		}
	}

	if cmUpdated {
		// sometimes an old cm resource version is returned ie get happens too fast - before server updates CM
		time.Sleep(time.Duration(1) * time.Second)
		cm := r.GetEffectiveConfigMap(instance, cheConfigMap.Name)
		cmResourceVersion := cm.ResourceVersion
		cheDeployment, err := deploy.NewCheDeployment(instance, cheImageRepo, cheImageTag, cmResourceVersion, isOpenShift)
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
	deployment, _ = r.GetEffectiveDeployment(instance, cheDeployment.Name)
	actualMemRequest := deployment.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
	actualMemLimit := deployment.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]
	limitStr := actualMemLimit.String()
	requestStr := actualMemRequest.String()
	desiredRequest := util.GetValue(instance.Spec.Server.ServerMemoryRequest, deploy.DefaultServerMemoryRequest)
	desiredLimit := util.GetValue(instance.Spec.Server.ServerMemoryLimit, deploy.DefaultServerMemoryLimit)
	if desiredRequest != requestStr || desiredLimit != limitStr {
		cheDeployment, err := deploy.NewCheDeployment(instance, cheImageRepo, cheImageTag, cmResourceVersion, isOpenShift)
		if err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		if err := controllerutil.SetControllerReference(instance, cheDeployment, r.scheme); err != nil {
			logrus.Errorf("An error occurred: %s", err)
		}
		logrus.Infof("Updating deployment %s with new memory settings. Request: %s, limit: %s", cheDeployment.Name, desiredRequest, desiredLimit)
		if err := r.client.Update(context.TODO(), cheDeployment); err != nil {
			logrus.Errorf("Failed to update deployment: %s", err)
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
