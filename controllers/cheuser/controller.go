package cheuser

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	org "github.com/eclipse-che/che-operator/api"
	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/controllers/che"
	"github.com/eclipse-che/che-operator/controllers/devworkspace"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	projectv1 "github.com/openshift/api/project/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type CheUserNamespaceReconciler struct {
	client         client.Client
	scheme         runtime.Scheme
	namespaceCache namespaceCache
}

var _ reconcile.Reconciler = (*CheUserNamespaceReconciler)(nil)

func NewReconciler() *CheUserNamespaceReconciler {
	return &CheUserNamespaceReconciler{namespaceCache: namespaceCache{knownNamespaces: make(map[string]namespaceInfo)}}
}

func (r *CheUserNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = *mgr.GetScheme()
	r.client = mgr.GetClient()
	r.namespaceCache.client = r.client

	var obj runtime.Object
	if infrastructure.IsOpenShift() {
		obj = &projectv1.Project{}
	} else {
		obj = &corev1.Namespace{}
	}

	ctx := context.Background()
	bld := ctrl.NewControllerManagedBy(mgr).
		For(obj).
		Watches(&source.Kind{Type: &corev1.Secret{}}, r.convertToNamespaceRequest(ctx)).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, r.convertToNamespaceRequest(ctx)).
		Watches(&source.Kind{Type: &v1.CheCluster{}}, r.triggerAllNamespaces(ctx))

	return bld.Complete(r)
}

func (r *CheUserNamespaceReconciler) convertToNamespaceRequest(ctx context.Context) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mo handler.MapObject) []reconcile.Request {
			info, err := r.namespaceCache.GetNamespaceInfo(ctx, mo.Meta.GetNamespace())
			if err != nil || info == nil || info.OwnerUid == "" {
				return []reconcile.Request{}
			}

			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{Name: mo.Meta.GetNamespace()},
				},
			}
		}),
	}
}

func (r *CheUserNamespaceReconciler) triggerAllNamespaces(ctx context.Context) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mo handler.MapObject) []reconcile.Request {
			nss := r.namespaceCache.GetAllKnownNamespaces()
			ret := make([]reconcile.Request, len(nss))

			for _, ns := range nss {
				ret = append(ret, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: ns},
				})
			}

			return ret
		}),
	}
}

func (r *CheUserNamespaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	info, err := r.namespaceCache.ExamineNamespace(ctx, req.Name)
	if err != nil || info == nil {
		return ctrl.Result{}, err
	}

	checluster := findManagingCheCluster(*info.CheCluster)
	if checluster == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	if devworkspace.GetDevworkspaceState(&r.scheme, checluster) != devworkspace.DevworkspaceStateEnabled {
		return ctrl.Result{}, nil
	}

	// let's construct the deployContext to be able to use methods from v1 operator
	deployContext := &deploy.DeployContext{
		CheCluster: org.AsV1(checluster),
		ClusterAPI: deploy.ClusterAPI{
			Client:          r.client,
			NonCachedClient: r.client,
			DiscoveryClient: nil,
			Scheme:          &r.scheme,
		},
	}

	isSelfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isSelfSignedCertUsed {
		if err = r.reconcileSelfSignedCert(req.Name, checluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err = r.reconcileProxySettings(ctx, req.Name, checluster, deployContext); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func findManagingCheCluster(key types.NamespacedName) *v2alpha1.CheCluster {
	instances := devworkspace.GetCurrentCheClusterInstances()
	if len(instances) == 0 {
		return nil
	}

	if len(instances) == 1 {
		for k, v := range instances {
			if key.Name == "" || (key.Name == k.Name && key.Namespace == k.Namespace) {
				return &v
			}
			return nil
		}
	}

	ret := instances[key]

	return &ret
}

func (r *CheUserNamespaceReconciler) reconcileSelfSignedCert(targetNs string, checluster *v2alpha1.CheCluster) error {
	// TODO implement
	return nil
}

func (r *CheUserNamespaceReconciler) reconcileProxySettings(ctx context.Context, targetNs string, checluster *v2alpha1.CheCluster, deployContext *deploy.DeployContext) error {
	proxyConfig, err := che.GetProxyConfiguration(deployContext)
	if err != nil {
		return err
	}

	if proxyConfig == nil {
		return nil
	}

	proxySettings := map[string]string{}
	if proxyConfig.HttpProxy != "" {
		proxySettings["HTTP_PROXY"] = proxyConfig.HttpProxy
	}
	if proxyConfig.HttpsProxy != "" {
		proxySettings["HTTPS_PROXY"] = proxyConfig.HttpsProxy
	}
	if proxyConfig.NoProxy != "" {
		proxySettings["NO_PROXY"] = proxyConfig.NoProxy
	}

	key := client.ObjectKey{Name: checluster.Name + "-" + checluster.Namespace + "-proxy-settings", Namespace: targetNs}
	cfg := &corev1.ConfigMap{}
	exists := true
	if err := r.client.Get(ctx, key, cfg); err != nil {
		if errors.IsNotFound(err) {
			exists = false
		} else {
			return err
		}
	}

	if len(proxySettings) == 0 {
		if exists {
			if err := r.client.Delete(ctx, cfg); err != nil {
				return err
			}
		}
		return nil
	}

	requiredLabels := defaults.AddStandardLabelsForComponent(checluster, "workspace-settings", map[string]string{
		constants.DevWorkspaceMountLabel: "true",
	})
	requiredAnnos := map[string]string{
		constants.DevWorkspaceMountAsAnnotation: "env",
	}

	cfg = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        checluster.Name + "-" + checluster.Namespace + "-proxy-settings",
			Namespace:   targetNs,
			Labels:      requiredLabels,
			Annotations: requiredAnnos,
		},
		Data: proxySettings,
	}

	_, err = deploy.DoSync(deployContext, cfg)
	return err
}
