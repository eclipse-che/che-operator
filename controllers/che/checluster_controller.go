//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/consolelink"
	"github.com/eclipse-che/che-operator/pkg/deploy/dashboard"
	devworkspace "github.com/eclipse-che/che-operator/pkg/deploy/dev-workspace"
	"github.com/eclipse-che/che-operator/pkg/deploy/devfileregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	identityprovider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	imagepuller "github.com/eclipse-che/che-operator/pkg/deploy/image-puller"
	"github.com/eclipse-che/che-operator/pkg/deploy/migration"
	"github.com/eclipse-che/che-operator/pkg/deploy/pluginregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/postgres"
	"github.com/eclipse-che/che-operator/pkg/deploy/rbac"
	"github.com/eclipse-che/che-operator/pkg/deploy/server"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	discoveryClient  discovery.DiscoveryInterface
	reconcileManager *deploy.ReconcileManager
	// the namespace to which to limit the reconciliation. If empty, all namespaces are considered
	namespace string
}

// NewReconciler returns a new CheClusterReconciler
func NewReconciler(
	k8sclient client.Client,
	noncachedClient client.Client,
	discoveryClient discovery.DiscoveryInterface,
	scheme *k8sruntime.Scheme,
	namespace string) *CheClusterReconciler {

	reconcileManager := deploy.NewReconcileManager()

	// order does matter
	if !util.IsTestMode() {
		reconcileManager.RegisterReconciler(migration.NewMigrator())
		reconcileManager.RegisterReconciler(NewCheClusterValidator())
	}
	reconcileManager.RegisterReconciler(imagepuller.NewImagePuller())

	reconcileManager.RegisterReconciler(tls.NewCertificatesReconciler())
	reconcileManager.RegisterReconciler(tls.NewTlsSecretReconciler())
	reconcileManager.RegisterReconciler(devworkspace.NewDevWorkspaceReconciler())
	reconcileManager.RegisterReconciler(rbac.NewCheServerPermissionsReconciler())
	reconcileManager.RegisterReconciler(rbac.NewGatewayPermissionsReconciler())
	reconcileManager.RegisterReconciler(rbac.NewWorkspacePermissionsReconciler())
	reconcileManager.RegisterReconciler(server.NewDefaultValuesReconciler())

	// we have to expose che endpoint independently of syncing other server
	// resources since che host is used for dashboard deployment and che config map
	reconcileManager.RegisterReconciler(server.NewCheHostReconciler())
	reconcileManager.RegisterReconciler(postgres.NewPostgresReconciler())
	reconcileManager.RegisterReconciler(identityprovider.NewIdentityProviderReconciler())
	reconcileManager.RegisterReconciler(devfileregistry.NewDevfileRegistryReconciler())
	reconcileManager.RegisterReconciler(pluginregistry.NewPluginRegistryReconciler())
	reconcileManager.RegisterReconciler(dashboard.NewDashboardReconciler())
	reconcileManager.RegisterReconciler(gateway.NewGatewayReconciler())
	reconcileManager.RegisterReconciler(server.NewCheServerReconciler())

	if util.IsOpenShift4 {
		reconcileManager.RegisterReconciler(consolelink.NewConsoleLinkReconciler())
	}

	return &CheClusterReconciler{
		Scheme: scheme,
		Log:    ctrl.Log.WithName("controllers").WithName("CheCluster"),

		client:           k8sclient,
		nonCachedClient:  noncachedClient,
		discoveryClient:  discoveryClient,
		namespace:        namespace,
		reconcileManager: reconcileManager,
	}
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
		Watches(&source.Kind{Type: &rbacv1.Role{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &orgv1.CheCluster{},
		}).
		Watches(&source.Kind{Type: &rbacv1.RoleBinding{}}, &handler.EnqueueRequestForOwner{
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
		Client:           r.client,
		NonCachingClient: r.nonCachedClient,
		DiscoveryClient:  r.discoveryClient,
		Scheme:           r.Scheme,
	}

	// Fetch the CheCluster instance
	checluster, err := r.GetCR(req)

	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("CheCluster Custom Resource not found.")
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
		CheCluster: checluster,
	}

	// Read proxy configuration
	proxy, err := GetProxyConfiguration(deployContext)
	if err != nil {
		r.Log.Error(err, "Error on reading proxy configuration")
		return ctrl.Result{}, err
	}
	deployContext.Proxy = proxy

	// Detect whether self-signed certificate is used
	isSelfSignedCertificate, err := tls.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		r.Log.Error(err, "Failed to detect if self-signed certificate used.")
		return ctrl.Result{}, err
	}
	deployContext.IsSelfSignedCertificate = isSelfSignedCertificate

	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		result, done, err := r.reconcileManager.ReconcileAll(deployContext)
		if !done {
			return result, err
		} else {
			logrus.Info("Successfully reconciled.")
			return ctrl.Result{}, nil
		}
	} else {
		done := r.reconcileManager.FinalizeAll(deployContext)
		return ctrl.Result{Requeue: !done}, nil
	}
}

func (r *CheClusterReconciler) GetCR(request ctrl.Request) (*orgv1.CheCluster, error) {
	checluster := &orgv1.CheCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, checluster)
	return checluster, err
}
