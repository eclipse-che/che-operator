//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"time"

	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	imagepuller "github.com/eclipse-che/che-operator/pkg/deploy/image-puller"

	editorsdefinitions "github.com/eclipse-che/che-operator/pkg/deploy/editors-definitions"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	containerbuild "github.com/eclipse-che/che-operator/pkg/deploy/container-capabilities"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/consolelink"
	"github.com/eclipse-che/che-operator/pkg/deploy/dashboard"
	devworkspaceconfig "github.com/eclipse-che/che-operator/pkg/deploy/dev-workspace-config"
	"github.com/eclipse-che/che-operator/pkg/deploy/devfileregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	identityprovider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/deploy/migration"
	"github.com/eclipse-che/che-operator/pkg/deploy/pluginregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/postgres"
	"github.com/eclipse-che/che-operator/pkg/deploy/rbac"
	"github.com/eclipse-che/che-operator/pkg/deploy/server"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
	if !test.IsTestMode() {
		reconcileManager.RegisterReconciler(migration.NewMigrator())
		reconcileManager.RegisterReconciler(migration.NewCheClusterDefaultsCleaner())
		reconcileManager.RegisterReconciler(NewCheClusterValidator())
	}

	reconcileManager.RegisterReconciler(tls.NewCertificatesReconciler())
	reconcileManager.RegisterReconciler(tls.NewTlsSecretReconciler())
	reconcileManager.RegisterReconciler(devworkspaceconfig.NewDevWorkspaceConfigReconciler())
	reconcileManager.RegisterReconciler(rbac.NewGatewayPermissionsReconciler())

	// we have to expose che endpoint independently of syncing other server
	// resources since che host is used for dashboard deployment and che config map
	reconcileManager.RegisterReconciler(server.NewCheHostReconciler())
	reconcileManager.RegisterReconciler(postgres.NewPostgresReconciler())
	if infrastructure.IsOpenShift() {
		reconcileManager.RegisterReconciler(identityprovider.NewIdentityProviderReconciler())
	}
	reconcileManager.RegisterReconciler(devfileregistry.NewDevfileRegistryReconciler())
	reconcileManager.RegisterReconciler(pluginregistry.NewPluginRegistryReconciler())
	reconcileManager.RegisterReconciler(editorsdefinitions.NewEditorsDefinitionsReconciler())
	reconcileManager.RegisterReconciler(dashboard.NewDashboardReconciler())
	reconcileManager.RegisterReconciler(gateway.NewGatewayReconciler())
	reconcileManager.RegisterReconciler(server.NewCheServerReconciler())
	reconcileManager.RegisterReconciler(imagepuller.NewImagePuller())

	if infrastructure.IsOpenShift() {
		reconcileManager.RegisterReconciler(containerbuild.NewContainerBuildReconciler())
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
	var toTrustedBundleConfigMapRequestMapper handler.MapFunc = func(ctx context.Context, obj client.Object) []reconcile.Request {
		isTrusted, reconcileRequest := IsTrustedBundleConfigMap(r.client, r.namespace, obj)
		if isTrusted {
			return []reconcile.Request{reconcileRequest}
		}
		return []reconcile.Request{}
	}

	var toEclipseCheRelatedObjRequestMapper handler.MapFunc = func(ctx context.Context, obj client.Object) []reconcile.Request {
		isEclipseCheRelatedObj, reconcileRequest := IsEclipseCheRelatedObj(r.client, r.namespace, obj)
		if isEclipseCheRelatedObj {
			return []reconcile.Request{reconcileRequest}
		}
		return []reconcile.Request{}
	}

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&chev2.CheCluster{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&appsv1.Deployment{}).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(toTrustedBundleConfigMapRequestMapper),
		).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(toEclipseCheRelatedObjRequestMapper),
		).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(toEclipseCheRelatedObjRequestMapper),
		)

	if infrastructure.IsOpenShift() {
		bld.Owns(&routev1.Route{})
	} else {
		bld.Owns(&networking.Ingress{})
	}

	if r.namespace != "" {
		bld = bld.WithEventFilter(utils.InNamespaceEventFilter(r.namespace))
	}

	// Use controller.TypedOptions to allow to configure 2 controllers for same object being reconciled
	return bld.WithOptions(
		controller.TypedOptions[reconcile.Request]{
			SkipNameValidation: pointer.Bool(true),
		}).Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.5/pkg/reconcile
func (r *CheClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("checluster", req.NamespacedName)

	clusterAPI := chetypes.ClusterAPI{
		Client:                  r.client,
		NonCachingClient:        r.nonCachedClient,
		DiscoveryClient:         r.discoveryClient,
		Scheme:                  r.Scheme,
		ClientWrapper:           k8sclient.NewK8sClient(r.client, r.Scheme),
		NonCachingClientWrapper: k8sclient.NewK8sClient(r.nonCachedClient, r.Scheme),
	}

	// Fetch the CheCluster instance
	checluster, err := deploy.FindCheClusterCRInNamespace(r.client, req.NamespacedName.Namespace)
	if checluster == nil {
		r.Log.Info("CheCluster Custom Resource not found.")
		return ctrl.Result{}, nil
	} else if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	deployContext := &chetypes.DeployContext{
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
			if err := deploy.SetStatusDetails(deployContext, "", ""); err != nil {
				return ctrl.Result{}, err
			}
			logrus.Info("Successfully reconciled.")
			return ctrl.Result{}, nil
		}
	} else {
		deployContext.CheCluster.Status.ChePhase = chev2.ClusterPhasePendingDeletion
		_ = deploy.UpdateCheCRStatus(deployContext, "ChePhase", chev2.ClusterPhasePendingDeletion)

		if done := r.reconcileManager.FinalizeAll(deployContext); !done {
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil

		}
		return ctrl.Result{}, nil
	}
}
