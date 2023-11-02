//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"encoding/json"
	"fmt"
	"strconv"

	containerbuild "github.com/eclipse-che/che-operator/pkg/deploy/container-build"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
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
	// we're define these here because we're forced to use an older version
	// of devworkspace operator as our dependency due to different go version
	nodeSelectorAnnotation   = "controller.devfile.io/node-selector"
	podTolerationsAnnotation = "controller.devfile.io/pod-tolerations"
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
		Watches(&source.Kind{Type: &chev2.CheCluster{}}, r.triggerAllNamespaces())

	return bld.Complete(r)
}

func (r *CheUserNamespaceReconciler) watchRulesForSecrets(ctx context.Context) handler.EventHandler {
	rules := r.commonRules(ctx, constants.DefaultSelfSignedCertificateSecretName)
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
	return err == nil && info != nil && info.IsWorkspaceNamespace
}

func (r *CheUserNamespaceReconciler) triggerAllNamespaces() handler.EventHandler {
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
	list := chev2.CheClusterList{}
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

	if info == nil || !info.IsWorkspaceNamespace {
		// we're not handling this namespace
		return ctrl.Result{}, nil
	}

	checluster := findManagingCheCluster(*info.CheCluster)
	if checluster == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	// let's construct the deployContext to be able to use methods from v1 operator
	deployContext := &chetypes.DeployContext{
		CheCluster: checluster,
		ClusterAPI: chetypes.ClusterAPI{
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
		logrus.Errorf("Failed to reconcile trusted certificates into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileProxySettings(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile proxy settings into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileGitTlsCertificate(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile Che git TLS certificate  into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileIdleSettings(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile idle settings into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileNodeSelectorAndTolerations(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile the workspace pod node selector and tolerations in namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileSCCPrivileges(info.Username, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile the SCC privileges in namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func findManagingCheCluster(key types.NamespacedName) *chev2.CheCluster {
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

func (r *CheUserNamespaceReconciler) reconcileSelfSignedCert(ctx context.Context, deployContext *chetypes.DeployContext, targetNs string, checluster *chev2.CheCluster) error {
	if err := deleteLegacyObject("server-cert", &corev1.Secret{}, targetNs, checluster, deployContext); err != nil {
		return err
	}
	targetCertName := prefixedName("server-cert")

	delSecret := func() error {
		_, err := deploy.Delete(deployContext, client.ObjectKey{Name: targetCertName, Namespace: targetNs}, &corev1.Secret{})
		return err
	}

	cheCert := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: constants.DefaultSelfSignedCertificateSecretName, Namespace: checluster.Namespace}, cheCert); err != nil {
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
				dwconstants.DevWorkspaceMountLabel:       "true",
				dwconstants.DevWorkspaceWatchSecretLabel: "true",
			}),
			Annotations: map[string]string{
				dwconstants.DevWorkspaceMountAsAnnotation:   "file",
				dwconstants.DevWorkspaceMountPathAnnotation: "/tmp/che/secret/",
			},
		},
		Data: map[string][]byte{
			"ca.crt": cheCert.Data["ca.crt"],
		},
		Type:      cheCert.Type,
		Immutable: cheCert.Immutable,
	}

	_, err := deploy.Sync(deployContext, targetCert, deploy.SecretDiffOpts)
	return err
}

func (r *CheUserNamespaceReconciler) reconcileTrustedCerts(ctx context.Context, deployContext *chetypes.DeployContext, targetNs string, checluster *chev2.CheCluster) error {
	if err := deleteLegacyObject("trusted-ca-certs", &corev1.ConfigMap{}, targetNs, checluster, deployContext); err != nil {
		return err
	}
	targetConfigMapName := prefixedName("trusted-ca-certs")

	delConfigMap := func() error {
		_, err := deploy.Delete(deployContext, client.ObjectKey{Name: targetConfigMapName, Namespace: targetNs}, &corev1.ConfigMap{})
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
				dwconstants.DevWorkspaceMountLabel:          "true",
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
			}),
			Annotations: addToFirst(sourceMap.Annotations, map[string]string{
				dwconstants.DevWorkspaceMountAsAnnotation:   "file",
				dwconstants.DevWorkspaceMountPathAnnotation: "/public-certs",
			}),
		},
		Data: sourceMap.Data,
	}

	_, err := deploy.Sync(deployContext, targetMap, deploy.ConfigMapDiffOpts)
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

func (r *CheUserNamespaceReconciler) reconcileProxySettings(ctx context.Context, targetNs string, checluster *chev2.CheCluster, deployContext *chetypes.DeployContext) error {
	if err := deleteLegacyObject("proxy-settings", &corev1.ConfigMap{}, targetNs, checluster, deployContext); err != nil {
		return err
	}
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
		proxySettings["http_proxy"] = proxyConfig.HttpProxy
	}
	if proxyConfig.HttpsProxy != "" {
		proxySettings["HTTPS_PROXY"] = proxyConfig.HttpsProxy
		proxySettings["https_proxy"] = proxyConfig.HttpsProxy
	}
	if proxyConfig.NoProxy != "" {
		proxySettings["NO_PROXY"] = proxyConfig.NoProxy
		proxySettings["no_proxy"] = proxyConfig.NoProxy
	}

	key := client.ObjectKey{Name: prefixedName("proxy-settings"), Namespace: targetNs}
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
		dwconstants.DevWorkspaceMountLabel:          "true",
		dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
	})
	requiredAnnos := map[string]string{
		dwconstants.DevWorkspaceMountAsAnnotation: "env",
	}

	cfg = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        prefixedName("proxy-settings"),
			Namespace:   targetNs,
			Labels:      requiredLabels,
			Annotations: requiredAnnos,
		},
		Data: proxySettings,
	}

	_, err = deploy.Sync(deployContext, cfg, deploy.ConfigMapDiffOpts)
	return err
}

func (r *CheUserNamespaceReconciler) reconcileIdleSettings(ctx context.Context, targetNs string, checluster *chev2.CheCluster, deployContext *chetypes.DeployContext) error {

	if checluster.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling == nil && checluster.Spec.DevEnvironments.SecondsOfRunBeforeIdling == nil {
		return nil
	}
	configMapName := prefixedName("idle-settings")
	cfg := &corev1.ConfigMap{}

	requiredLabels := defaults.AddStandardLabelsForComponent(checluster, userSettingsComponentLabelValue, map[string]string{
		dwconstants.DevWorkspaceMountLabel:          "true",
		dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
	})
	requiredAnnos := map[string]string{
		dwconstants.DevWorkspaceMountAsAnnotation: "env",
	}

	data := map[string]string{}

	if checluster.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling != nil {
		data["SECONDS_OF_DW_INACTIVITY_BEFORE_IDLING"] = strconv.FormatInt(int64(*checluster.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling), 10)
	}

	if checluster.Spec.DevEnvironments.SecondsOfRunBeforeIdling != nil {
		data["SECONDS_OF_DW_RUN_BEFORE_IDLING"] = strconv.FormatInt(int64(*checluster.Spec.DevEnvironments.SecondsOfRunBeforeIdling), 10)
	}

	cfg = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        configMapName,
			Namespace:   targetNs,
			Labels:      requiredLabels,
			Annotations: requiredAnnos,
		},
		Data: data,
	}
	_, err := deploy.Sync(deployContext, cfg, deploy.ConfigMapDiffOpts)
	return err
}

func (r *CheUserNamespaceReconciler) reconcileGitTlsCertificate(ctx context.Context, targetNs string, checluster *chev2.CheCluster, deployContext *chetypes.DeployContext) error {
	if err := deleteLegacyObject("git-tls-creds", &corev1.ConfigMap{}, targetNs, checluster, deployContext); err != nil {
		return err
	}
	targetName := prefixedName("git-tls-creds")
	delConfigMap := func() error {
		_, err := deploy.Delete(deployContext, client.ObjectKey{Name: targetName, Namespace: targetNs}, &corev1.ConfigMap{})
		return err
	}

	if checluster.Spec.DevEnvironments.TrustedCerts == nil || checluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName == "" {
		return delConfigMap()
	}

	gitCert := &corev1.ConfigMap{}

	if err := deployContext.ClusterAPI.Client.Get(ctx, client.ObjectKey{Name: checluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName, Namespace: checluster.Namespace}, gitCert); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return delConfigMap()
	}

	if gitCert.Data["ca.crt"] == "" {
		return delConfigMap()
	}

	target := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels: defaults.AddStandardLabelsForComponent(checluster, userSettingsComponentLabelValue, map[string]string{
				dwconstants.DevWorkspaceGitTLSLabel:         "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
			}),
		},
		Data: map[string]string{
			"certificate": gitCert.Data["ca.crt"],
		},
	}

	if gitCert.Data["githost"] != "" {
		target.Data["host"] = gitCert.Data["githost"]
	}

	_, err := deploy.Sync(deployContext, &target, deploy.ConfigMapDiffOpts)
	return err
}

func (r *CheUserNamespaceReconciler) reconcileNodeSelectorAndTolerations(ctx context.Context, targetNs string, checluster *chev2.CheCluster, deployContext *chetypes.DeployContext) error {
	ns := &corev1.Namespace{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: targetNs}, ns); err != nil {
		return err
	}

	nodeSelector := ""
	tolerations := ""

	if len(checluster.Spec.DevEnvironments.NodeSelector) != 0 {
		serialized, err := json.Marshal(checluster.Spec.DevEnvironments.NodeSelector)
		if err != nil {
			return err
		}

		nodeSelector = string(serialized)
	}

	if len(checluster.Spec.DevEnvironments.Tolerations) != 0 {
		serialized, err := json.Marshal(checluster.Spec.DevEnvironments.Tolerations)
		if err != nil {
			return err
		}

		tolerations = string(serialized)
	}

	annos := ns.GetAnnotations()
	if annos == nil {
		annos = map[string]string{}
	}

	if len(nodeSelector) == 0 {
		delete(annos, nodeSelectorAnnotation)
	} else {
		annos[nodeSelectorAnnotation] = nodeSelector
	}

	if len(tolerations) == 0 {
		delete(annos, podTolerationsAnnotation)
	} else {
		annos[podTolerationsAnnotation] = tolerations
	}

	ns.SetAnnotations(annos)

	return r.client.Update(ctx, ns)
}

func (r *CheUserNamespaceReconciler) reconcileSCCPrivileges(username string, targetNs string, checluster *chev2.CheCluster, deployContext *chetypes.DeployContext) error {
	delRoleBinding := func() error {
		_, err := deploy.Delete(
			deployContext,
			types.NamespacedName{Name: containerbuild.GetUserSccRbacResourcesName(), Namespace: targetNs},
			&rbacv1.RoleBinding{})
		return err
	}

	if !checluster.IsContainerBuildCapabilitiesEnabled() {
		return delRoleBinding()
	}

	if username == "" {
		_ = delRoleBinding()
		return fmt.Errorf("unknown user for %s namespace", targetNs)
	}

	rb := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      containerbuild.GetUserSccRbacResourcesName(),
			Namespace: targetNs,
			Labels:    map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     containerbuild.GetUserSccRbacResourcesName(),
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: "rbac.authorization.k8s.io",
				Name:     username,
			},
		},
	}

	if _, err := deploy.Sync(deployContext, rb, deploy.RollBindingDiffOpts); err != nil {
		return err
	}

	return nil
}

func prefixedName(name string) string {
	return "che-" + name
}

// Deletes object with a legacy name to avoid mounting several ones under the same path
// See https://github.com/eclipse/che/issues/21385
func deleteLegacyObject(name string, objectMeta client.Object, targetNs string, checluster *chev2.CheCluster, deployContext *chetypes.DeployContext) error {
	legacyPrefixedName := checluster.Name + "-" + checluster.Namespace + "-" + name
	key := client.ObjectKey{Name: legacyPrefixedName, Namespace: targetNs}

	err := deployContext.ClusterAPI.Client.Get(context.TODO(), key, objectMeta)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = deployContext.ClusterAPI.Client.Delete(context.TODO(), objectMeta)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	logrus.Infof("Deleted legacy workspace object: %s name: %s, namespace: %s", deploy.GetObjectType(objectMeta), legacyPrefixedName, targetNs)
	return nil
}
