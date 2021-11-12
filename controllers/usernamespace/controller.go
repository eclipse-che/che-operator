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

package usernamespace

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	"github.com/eclipse-che/che-operator/pkg/util"

	"github.com/devfile/devworkspace-operator/pkg/constants"
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

type eventRule struct {
	check      func(metav1.Object) bool
	namespaces func(metav1.Object) []string
}

var _ reconcile.Reconciler = (*CheUserNamespaceReconciler)(nil)

func NewReconciler() *CheUserNamespaceReconciler {
	return &CheUserNamespaceReconciler{namespaceCache: *NewNamespaceCache()}
}

func (r *CheUserNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	r.client = mgr.GetClient()
	r.namespaceCache.client = r.client

	var obj client.Object
	if util.IsOpenShift4 {
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
	rules := r.commonRules(ctx, deploy.CheTLSSelfSignedCertificateSecretName)
	return handler.EnqueueRequestsFromMapFunc(
		handler.MapFunc(func(obj client.Object) []reconcile.Request {
			return asReconcileRequestsForNamespaces(obj, rules)
		}))
}

func asReconcileRequestsForNamespaces(obj metav1.Object, rules []eventRule) []reconcile.Request {
	for _, r := range rules {
		if r.check(obj) {
			nss := r.namespaces(obj)
			ret := make([]reconcile.Request, len(nss))
			for i, n := range nss {
				ret[i] = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: n,
					},
				}
			}

			return ret
		}
	}

	return []reconcile.Request{}
}

func (r *CheUserNamespaceReconciler) commonRules(ctx context.Context, namesInCheClusterNamespace ...string) []eventRule {
	return []eventRule{
		{
			check: func(o metav1.Object) bool {
				return isLabeledAsUserSettings(o) && r.isInManagedNamespace(ctx, o)
			},
			namespaces: func(o metav1.Object) []string { return []string{o.GetNamespace()} },
		},
		{
			check: func(o metav1.Object) bool {
				return r.hasNameAndIsCollocatedWithCheCluster(ctx, o, namesInCheClusterNamespace...)
			},
			namespaces: func(o metav1.Object) []string { return r.namespaceCache.GetAllKnownNamespaces() },
		},
	}
}

func (r *CheUserNamespaceReconciler) watchRulesForConfigMaps(ctx context.Context) handler.EventHandler {
	rules := r.commonRules(ctx, tls.CheAllCACertsConfigMapName)
	return handler.EnqueueRequestsFromMapFunc(
		handler.MapFunc(func(obj client.Object) []reconcile.Request {
			return asReconcileRequestsForNamespaces(obj, rules)
		}))
}

func (r *CheUserNamespaceReconciler) hasNameAndIsCollocatedWithCheCluster(ctx context.Context, obj metav1.Object, names ...string) bool {
	for _, n := range names {
		if obj.GetName() == n && r.hasCheCluster(ctx, obj.GetNamespace()) {
			return true
		}
	}

	return false
}

func isLabeledAsUserSettings(obj metav1.Object) bool {
	return obj.GetLabels()["app.kubernetes.io/component"] == userSettingsComponentLabelValue
}

func (r *CheUserNamespaceReconciler) isInManagedNamespace(ctx context.Context, obj metav1.Object) bool {
	info, err := r.namespaceCache.GetNamespaceInfo(ctx, obj.GetNamespace())
	return err == nil && info != nil && info.OwnerUid != ""
}

func (r *CheUserNamespaceReconciler) triggerAllNamespaces(ctx context.Context) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		handler.MapFunc(func(obj client.Object) []reconcile.Request {
			nss := r.namespaceCache.GetAllKnownNamespaces()
			ret := make([]reconcile.Request, len(nss))

			for _, ns := range nss {
				ret = append(ret, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: ns},
				})
			}

			return ret
		}),
	)
}

func (r *CheUserNamespaceReconciler) hasCheCluster(ctx context.Context, namespace string) bool {
	list := v1.CheClusterList{}
	if err := r.client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return false
	}

	return len(list.Items) > 0
}

func (r *CheUserNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	if devworkspace.GetDevWorkspaceState(r.scheme, checluster) != devworkspace.EnabledState {
		return ctrl.Result{}, nil
	}

	// let's construct the deployContext to be able to use methods from v1 operator
	deployContext := &deploy.DeployContext{
		CheCluster: org.AsV1(checluster),
		ClusterAPI: deploy.ClusterAPI{
			Client:           r.client,
			NonCachingClient: r.client,
			DiscoveryClient:  nil,
			Scheme:           r.scheme,
		},
	}

	if err = r.reconcileSelfSignedCert(ctx, deployContext, req.Name, checluster); err != nil {
		logrus.Errorf("Failed to reconcile self-signed certificate into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileTrustedCerts(ctx, deployContext, req.Name, checluster); err != nil {
		logrus.Errorf("Failed to reconcile self-signed certificate into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
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
	targetCertName := prefixedName(checluster, "server-cert")

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
		Type:      cheCert.Type,
		Immutable: cheCert.Immutable,
	}

	_, err := deploy.DoSync(deployContext, targetCert, deploy.SecretDiffOpts)
	return err
}

func (r *CheUserNamespaceReconciler) reconcileTrustedCerts(ctx context.Context, deployContext *deploy.DeployContext, targetNs string, checluster *v2alpha1.CheCluster) error {
	targetConfigMapName := prefixedName(checluster, "trusted-ca-certs")

	delConfigMap := func() error {
		_, err := deploy.Delete(deployContext, client.ObjectKey{Name: targetConfigMapName, Namespace: targetNs}, &corev1.Secret{})
		return err
	}

	sourceMap := &corev1.ConfigMap{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: tls.CheAllCACertsConfigMapName, Namespace: checluster.Namespace}, sourceMap); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		return delConfigMap()
	}

	targetMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetConfigMapName,
			Namespace: targetNs,
			Labels: defaults.AddStandardLabelsForComponent(checluster, userSettingsComponentLabelValue, map[string]string{
				constants.DevWorkspaceMountLabel: "true",
			}),
			Annotations: addToFirst(sourceMap.Annotations, map[string]string{
				constants.DevWorkspaceMountAsAnnotation:   "file",
				constants.DevWorkspaceMountPathAnnotation: "/public-certs",
			}),
		},
		Data: sourceMap.Data,
	}

	_, err := deploy.DoSync(deployContext, targetMap, deploy.ConfigMapDiffOpts)
	return err
}

func addToFirst(first map[string]string, second map[string]string) map[string]string {
	if first == nil {
		first = map[string]string{}
	}
	for k, v := range second {
		first[k] = v
	}

	return first
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
