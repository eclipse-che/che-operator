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
	"github.com/sirupsen/logrus"
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

const (
	userSettingsComponentLabelValue = "user-settings"
)

type CheUserNamespaceReconciler struct {
	client         client.Client
	scheme         *runtime.Scheme
	namespaceCache namespaceCache
}

var _ reconcile.Reconciler = (*CheUserNamespaceReconciler)(nil)

func NewReconciler() *CheUserNamespaceReconciler {
	return &CheUserNamespaceReconciler{namespaceCache: *NewNamespaceCache()}
}

func (r *CheUserNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
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
		Watches(&source.Kind{Type: &corev1.Secret{}}, r.watchRulesForSecrets(ctx)).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, r.watchRulesForConfigMaps(ctx)).
		Watches(&source.Kind{Type: &v1.CheCluster{}}, r.triggerAllNamespaces(ctx))

	return bld.Complete(r)
}

func (r *CheUserNamespaceReconciler) watchRulesForSecrets(ctx context.Context) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mo handler.MapObject) []reconcile.Request {
			if isLabeledAsUserSettings(mo.Meta) && r.isInManagedNamespace(ctx, mo.Meta) {
				return asReconcileRequestForNamespace(mo.Meta)
			} else {
				// need to watch for self-signed-certificate in a namespace with some checluster resource
				if mo.Meta.GetName() == "self-signed-certificate" && r.hasCheCluster(ctx, mo.Meta.GetNamespace()) {
					return asReconcileRequestForNamespace(mo.Meta)
				} else {
					return []reconcile.Request{}
				}
			}
		}),
	}
}

func (r *CheUserNamespaceReconciler) watchRulesForConfigMaps(ctx context.Context) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mo handler.MapObject) []reconcile.Request {
			if isLabeledAsUserSettings(mo.Meta) && r.isInManagedNamespace(ctx, mo.Meta) {
				return asReconcileRequestForNamespace(mo.Meta)
			} else {
				return []reconcile.Request{}
			}
		}),
	}
}

func isLabeledAsUserSettings(obj metav1.Object) bool {
	return obj.GetLabels()["app.kubernetes.io/component"] == userSettingsComponentLabelValue
}

func (r *CheUserNamespaceReconciler) isInManagedNamespace(ctx context.Context, obj metav1.Object) bool {
	info, err := r.namespaceCache.GetNamespaceInfo(ctx, obj.GetNamespace())
	return err == nil && info != nil && info.OwnerUid != ""
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

func (r *CheUserNamespaceReconciler) hasCheCluster(ctx context.Context, namespace string) bool {
	list := v1.CheClusterList{}
	if err := r.client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return false
	}

	return len(list.Items) > 0
}

func asReconcileRequestForNamespace(obj metav1.Object) []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{Name: obj.GetNamespace()},
		},
	}
}

func (r *CheUserNamespaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	info, err := r.namespaceCache.ExamineNamespace(ctx, req.Name)
	if err != nil {
		logrus.Errorf("Failed to examine namespace %s for presence of Che user info labels: %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if info == nil || info.OwnerUid == "" {
		// we're not handling this namespace
		return ctrl.Result{}, nil
	}

	checluster := findManagingCheCluster(*info.CheCluster)
	if checluster == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	if devworkspace.GetDevworkspaceState(r.scheme, checluster) != devworkspace.DevworkspaceStateEnabled {
		return ctrl.Result{}, nil
	}

	// let's construct the deployContext to be able to use methods from v1 operator
	deployContext := &deploy.DeployContext{
		CheCluster: org.AsV1(checluster),
		ClusterAPI: deploy.ClusterAPI{
			Client:          r.client,
			NonCachedClient: r.client,
			DiscoveryClient: nil,
			Scheme:          r.scheme,
		},
	}

	isSelfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		logrus.Errorf("Failed to figure out whether the configured certificate is self-signed: %v", err)
		return ctrl.Result{}, err
	}

	if isSelfSignedCertUsed {
		if err = r.reconcileSelfSignedCert(ctx, deployContext, req.Name, checluster); err != nil {
			logrus.Errorf("Failed to reconcile self-signed certificate into namespace '%s': %v", req.Name, err)
			return ctrl.Result{}, err
		}
	}

	if err = r.reconcileProxySettings(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile proxy settings into namespace '%s': %v", req.Name, err)
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

	ret, ok := instances[key]

	if ok {
		return &ret
	} else {
		return nil
	}
}

func (r *CheUserNamespaceReconciler) reconcileSelfSignedCert(ctx context.Context, deployContext *deploy.DeployContext, targetNs string, checluster *v2alpha1.CheCluster) error {
	targetCertName := prefixedName(checluster, "cert")

	delSecret := func() error {
		_, err := deploy.Delete(deployContext, client.ObjectKey{Name: targetCertName, Namespace: targetNs}, &corev1.Secret{})
		return err
	}

	cheCert := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: deploy.CheTLSSelfSignedCertificateSecretName, Namespace: checluster.Namespace}, cheCert); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// There is not self-signed cert in the namespace of the checluster, so we have nothing to copy around
		return delSecret()
	}

	if _, ok := cheCert.Data["ca.crt"]; !ok {
		// the secret doesn't contain the certificate. bail out.
		return delSecret()
	}

	targetCert := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetCertName,
			Namespace: targetNs,
			Labels: defaults.AddStandardLabelsForComponent(checluster, userSettingsComponentLabelValue, map[string]string{
				constants.DevWorkspaceMountLabel: "true",
			}),
			Annotations: map[string]string{
				constants.DevWorkspaceMountAsAnnotation:   "file",
				constants.DevWorkspaceMountPathAnnotation: "/tmp/che/secret/",
			},
		},
		Data: map[string][]byte{
			"ca.crt": cheCert.Data["ca.crt"],
		},
	}

	_, err := deploy.DoSync(deployContext, targetCert, deploy.SecretDiffOpts)
	return err
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

	key := client.ObjectKey{Name: prefixedName(checluster, "proxy-settings"), Namespace: targetNs}
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

	requiredLabels := defaults.AddStandardLabelsForComponent(checluster, userSettingsComponentLabelValue, map[string]string{
		constants.DevWorkspaceMountLabel: "true",
	})
	requiredAnnos := map[string]string{
		constants.DevWorkspaceMountAsAnnotation: "env",
	}

	cfg = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        prefixedName(checluster, "proxy-settings"),
			Namespace:   targetNs,
			Labels:      requiredLabels,
			Annotations: requiredAnnos,
		},
		Data: proxySettings,
	}

	_, err = deploy.DoSync(deployContext, cfg, deploy.ConfigMapDiffOpts)
	return err
}

func prefixedName(checluster *v2alpha1.CheCluster, name string) string {
	return checluster.Name + "-" + checluster.Namespace + "-" + name
}
