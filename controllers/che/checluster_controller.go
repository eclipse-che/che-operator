//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	agentsandbox "github.com/eclipse-che/che-operator/pkg/deploy/agent-sandbox"
	"github.com/eclipse-che/che-operator/pkg/deploy/metrics"
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	"github.com/eclipse-che/che-operator/pkg/reconciler/consolelink"
	"github.com/eclipse-che/che-operator/pkg/reconciler/networkpolicy"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/deploy/devworkspace"
	imagepuller "github.com/eclipse-che/che-operator/pkg/deploy/image-puller"

	editorsdefinitions "github.com/eclipse-che/che-operator/pkg/deploy/editors-definitions"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	containerbuild "github.com/eclipse-che/che-operator/pkg/deploy/container-capabilities"
	"github.com/eclipse-che/che-operator/pkg/deploy/dashboard"
	"github.com/eclipse-che/che-operator/pkg/deploy/devfileregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	identityprovider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/deploy/migration"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	openvsxdatabase "github.com/eclipse-che/che-operator/pkg/deploy/openvsx/openvsx-database"
	openvsxserver "github.com/eclipse-che/che-operator/pkg/deploy/openvsx/openvsx-server"
	"github.com/eclipse-che/che-operator/pkg/deploy/pluginregistry"
	"github.com/eclipse-che/che-operator/pkg/deploy/rbac"
	"github.com/eclipse-che/che-operator/pkg/deploy/server"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/go-logr/logr"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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
	// to simply read objects that we don't intend
	// to further watch
	nonCachedClient client.Client
	// A discovery client to check for the existence of certain APIs registered
	// in the API Server
	discoveryClient   discovery.DiscoveryInterface
	reconcilerManager *reconciler.ReconcilerManager
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

	reconcilerManager := reconciler.NewReconcilerManager()

	// order does matter
	reconcilerManager.AddReconciler(migration.NewMigrator())
	reconcilerManager.AddReconciler(migration.NewCheClusterDefaultsCleaner())
	reconcilerManager.AddReconciler(NewCheClusterValidator())
	reconcilerManager.AddReconciler(tls.NewCertificatesReconciler())
	reconcilerManager.AddReconciler(tls.NewTlsSecretReconciler())
	reconcilerManager.AddReconciler(devworkspace.NewDevWorkspaceConfigReconciler())
	reconcilerManager.AddReconciler(rbac.NewGatewayPermissionsReconciler())
	reconcilerManager.AddReconciler(&networkpolicy.NetworkPoliciesReconciler{})

	// we have to expose che endpoint independently of syncing other server
	// resources since che host is used for dashboard deployment and che config map
	reconcilerManager.AddReconciler(server.NewCheHostReconciler())
	reconcilerManager.AddReconciler(server.NewBaseDomainReconciler())
	if infrastructure.IsOpenShiftOAuthEnabled() {
		reconcilerManager.AddReconciler(identityprovider.NewIdentityProviderReconciler())
	}
	reconcilerManager.AddReconciler(devfileregistry.NewDevfileRegistryReconciler())
	reconcilerManager.AddReconciler(pluginregistry.NewPluginRegistryReconciler())
	reconcilerManager.AddReconciler(openvsx.NewOpenVSXSecretReconciler())
	reconcilerManager.AddReconciler(openvsxdatabase.NewOpenVSXDatabaseReconciler())
	reconcilerManager.AddReconciler(openvsxserver.NewOpenVSXServerReconciler())
	reconcilerManager.AddReconciler(editorsdefinitions.NewEditorsDefinitionsReconciler())
	reconcilerManager.AddReconciler(dashboard.NewDashboardReconciler())
	reconcilerManager.AddReconciler(gateway.NewGatewayReconciler())
	reconcilerManager.AddReconciler(server.NewCheServerReconciler())
	reconcilerManager.AddReconciler(imagepuller.NewImagePuller())

	if infrastructure.IsOpenShift() {
		reconcilerManager.AddReconciler(containerbuild.NewContainerCapabilitiesReconciler())
	}

	if infrastructure.IsConsoleLinkEnabled() {
		reconcilerManager.AddReconciler(consolelink.NewReconciler())
	}

	reconcilerManager.AddReconciler(agentsandbox.NewAgentSandboxReconciler())

	if infrastructure.IsServiceMonitorEnabled() {
		reconcilerManager.AddReconciler(metrics.NewMetricsReconciler())
	}

	return &CheClusterReconciler{
		Scheme: scheme,
		Log:    ctrl.Log.WithName("controllers").WithName("CheCluster"),

		client:            k8sclient,
		nonCachedClient:   noncachedClient,
		discoveryClient:   discoveryClient,
		namespace:         namespace,
		reconcilerManager: reconcilerManager,
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

	var toOperatorManagedObjRequestMapper handler.MapFunc = func(ctx context.Context, obj client.Object) []reconcile.Request {
		isOperatorManagedObj, reconcileRequest := IsOperatorManaged(r.client, r.namespace, obj)
		if isOperatorManagedObj {
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
		Owns(&batchv1.CronJob{}).
		Owns(&networking.NetworkPolicy{}).
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

	if infrastructure.IsConsoleLinkEnabled() {
		bld.Watches(&consolev1.ConsoleLink{},
			handler.EnqueueRequestsFromMapFunc(toOperatorManagedObjRequestMapper),
		)
	}

	if infrastructure.IsServiceMonitorEnabled() {
		bld.Owns(&monitoringv1.ServiceMonitor{})
	}

	if r.namespace != "" {
		bld = bld.WithEventFilter(utils.InNamespaceEventFilter(r.namespace))
	}

	// Use controller.TypedOptions to allow to configure 2 controllers for same object being reconciled
	return bld.WithOptions(
		controller.TypedOptions[reconcile.Request]{
			SkipNameValidation: ptr.To(true),
			UsePriorityQueue:   ptr.To(false),
		}).Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.5/pkg/reconcile
func (r *CheClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("checluster", req.NamespacedName)

	cheCluster := &chev2.CheCluster{}
	err := r.client.Get(ctx, req.NamespacedName, cheCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get CheCluster")
		return ctrl.Result{}, err
	}

	cheCtx := &chetypes.CheContext{
		Context:    ctx,
		CheCluster: cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:                  r.client,
			NonCachingClient:        r.nonCachedClient,
			DiscoveryClient:         r.discoveryClient,
			Scheme:                  r.Scheme,
			ClientWrapper:           clientwrapper.New(r.client, r.Scheme),
			NonCachingClientWrapper: clientwrapper.New(r.nonCachedClient, r.Scheme),
		},
	}

	// Ensure the CheCluster is not deleted until the operator
	// cleans up all related resources
	if cheCtx.CheCluster.DeletionTimestamp.IsZero() {
		if err := resources.CheCluster.AddFinalizer(cheCtx, constants.Finalizer); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resolve proxy configuration
	cheCtx.Proxy, err = GetProxyConfiguration(cheCtx)
	if err != nil {
		r.Log.Error(err, "Error on reading proxy configuration")
		return ctrl.Result{}, err
	}

	// Resolve authentication configuration
	cheCtx.Authentication, err = ResolveAuthentication(cheCtx)
	if err != nil {
		r.Log.Error(err, "Error on resolving authentication")
		return ctrl.Result{}, err
	}

	cheCtx.DWONamespace, err = devworkspace.GetDevWorkspaceOperatorNamespace(ctx, cheCtx.ClusterAPI.ClientWrapper)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get DevWorkspaceOperator namespace: %w", err)
	}
	if cheCtx.DWONamespace == "" {
		r.Log.Info("DevWorkspaceOperator namespace not found, requeuing.")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Detect whether self-signed certificate is used
	cheCtx.IsSelfSignedCertificate, err = tls.IsSelfSignedCertificateUsed(cheCtx)
	if err != nil {
		r.Log.Error(err, "Failed to detect if self-signed certificate used.")
		return ctrl.Result{}, err
	}

	if cheCtx.CheCluster.DeletionTimestamp.IsZero() {
		result, done, err := r.reconcilerManager.ReconcileAll(cheCtx)
		if done {
			cheCtx.CheCluster.Status.Reason = ""
			cheCtx.CheCluster.Status.Message = ""
			if err := resources.CheCluster.SyncStatus(cheCtx); err != nil {
				return ctrl.Result{}, err
			}

			r.Log.Info("Successfully reconciled.")
			return ctrl.Result{}, nil
		}
		if err != nil {
			errMsg := "Failed to reconcile CheCluster resources. The installation is not completed. Check operator logs for details."

			cheCtx.CheCluster.Status.Message = errMsg
			cheCtx.CheCluster.Status.Reason = constants.InstallOrUpdateFailed
			if err := resources.CheCluster.SyncStatus(cheCtx); err != nil {
				return ctrl.Result{}, err
			}

			return result, err
		}

		return result, nil
	}

	cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhasePendingDeletion
	if err := resources.CheCluster.SyncStatus(cheCtx); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcilerManager.FinalizeAll(cheCtx); err != nil {
		return ctrl.Result{}, err
	}

	// Removes remaining finalizers, which prevent CheCluster from deletion
	if err := resources.CheCluster.RemoveAllFinalizers(cheCtx); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
