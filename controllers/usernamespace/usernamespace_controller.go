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

package usernamespace

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	containercapabilties "github.com/eclipse-che/che-operator/pkg/deploy/container-capabilities"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/controllers/che"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
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
)

const (
	userSettingsComponentLabelValue = "user-settings"
	// we're define these here because we're forced to use an older version
	// of devworkspace operator as our dependency due to different go version
	nodeSelectorAnnotation   = "controller.devfile.io/node-selector"
	podTolerationsAnnotation = "controller.devfile.io/pod-tolerations"
)

type CheUserNamespaceReconciler struct {
	scheme                 *runtime.Scheme
	client                 client.Client
	nonCachedClient        client.Client
	clientWrapper          *k8sclient.K8sClientWrapper
	nonCachedClientWrapper *k8sclient.K8sClientWrapper
	namespaceCache         *namespacecache.NamespaceCache
}

var _ reconcile.Reconciler = (*CheUserNamespaceReconciler)(nil)

func NewCheUserNamespaceReconciler(
	client client.Client,
	noncachedClient client.Client,
	scheme *runtime.Scheme,
	namespaceCache *namespacecache.NamespaceCache) *CheUserNamespaceReconciler {

	return &CheUserNamespaceReconciler{
		scheme:                 scheme,
		client:                 client,
		nonCachedClient:        noncachedClient,
		clientWrapper:          k8sclient.NewK8sClient(client, scheme),
		nonCachedClientWrapper: k8sclient.NewK8sClient(noncachedClient, scheme),
		namespaceCache:         namespaceCache,
	}
}

func (r *CheUserNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var obj client.Object
	if infrastructure.IsOpenShift() {
		obj = &projectv1.Project{}
	} else {
		obj = &corev1.Namespace{}
	}

	ctx := context.Background()
	bld := ctrl.NewControllerManagedBy(mgr).
		For(obj).
		Watches(&corev1.Secret{}, r.watchRulesForSecrets(ctx)).
		Watches(&corev1.ConfigMap{}, r.watchRulesForConfigMaps(ctx)).
		Watches(&chev2.CheCluster{}, r.triggerAllNamespaces())

	// Use controller.TypedOptions to allow to configure 2 controllers for same object being reconciled
	return bld.WithOptions(
		controller.TypedOptions[reconcile.Request]{
			SkipNameValidation: pointer.Bool(true),
		}).Complete(r)
}

func (r *CheUserNamespaceReconciler) watchRulesForSecrets(ctx context.Context) handler.EventHandler {
	rules := r.commonRules(ctx, constants.DefaultSelfSignedCertificateSecretName)
	return handler.EnqueueRequestsFromMapFunc(
		handler.MapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return namespacecache.AsReconcileRequestsForNamespaces(obj, rules)
		}))
}

func (r *CheUserNamespaceReconciler) commonRules(ctx context.Context, namesInCheClusterNamespace ...string) []namespacecache.EventRule {
	return []namespacecache.EventRule{
		{
			Check: func(o metav1.Object) bool {
				return isLabeledAsUserSettings(o) && r.isInManagedNamespace(ctx, o)
			},
			Namespaces: func(o metav1.Object) []string { return []string{o.GetNamespace()} },
		},
		{
			Check: func(o metav1.Object) bool {
				return r.hasNameAndIsCollocatedWithCheCluster(ctx, o, namesInCheClusterNamespace...)
			},
			Namespaces: func(o metav1.Object) []string { return r.namespaceCache.GetAllKnownNamespaces() },
		},
	}
}

func (r *CheUserNamespaceReconciler) watchRulesForConfigMaps(ctx context.Context) handler.EventHandler {
	rules := r.commonRules(ctx, tls.CheMergedCABundleCertsCMName)
	return handler.EnqueueRequestsFromMapFunc(
		handler.MapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return namespacecache.AsReconcileRequestsForNamespaces(obj, rules)
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
		handler.MapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
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
	if req.Name == "" {
		return ctrl.Result{}, nil
	}

	info, err := r.namespaceCache.ExamineNamespace(ctx, req.Name)
	if err != nil {
		logrus.Errorf("Failed to examine namespace %s for presence of Che user info labels: %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if info == nil || !info.IsWorkspaceNamespace {
		// we're not handling this namespace
		return ctrl.Result{}, nil
	}

	checluster, err := deploy.FindCheClusterCRInNamespace(r.client, "")
	if checluster == nil || err != nil {
		// CheCluster is not found or error occurred, requeue the request
		return ctrl.Result{}, err
	}

	// let's construct the deployContext to be able to use methods from v1 operator
	deployContext := &chetypes.DeployContext{
		CheCluster: checluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:           r.client,
			NonCachingClient: r.nonCachedClient,
			Scheme:           r.scheme,
		},
	}

	// Deprecated [CRW-6792].
	// All certificates are mounted into /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
	// and automatically added to the system trust store.
	// TODO remove in the future.
	if err = r.reconcileSelfSignedCert(ctx, deployContext, req.Name, checluster); err != nil {
		logrus.Errorf("Failed to reconcile self-signed certificate into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	// Deprecated [CRW-6792].
	// All certificates are mounted into /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
	// and automatically added to the system trust store.
	// TODO remove in the future.
	if err = r.reconcileTrustedCerts(ctx, deployContext, req.Name, checluster); err != nil {
		logrus.Errorf("Failed to reconcile trusted certificates into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	// Deprecated [CRW-6792].
	// All certificates are mounted into /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
	// and automatically added to the system trust store.
	// TODO remove in the future.
	if err = r.reconcileGitTlsCertificate(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile Che git TLS certificate  into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileUserSettings(deployContext, req.Name, checluster); err != nil {
		logrus.Errorf("Failed to reconcile user settings into namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileNodeSelectorAndTolerations(ctx, req.Name, checluster, deployContext); err != nil {
		logrus.Errorf("Failed to reconcile the workspace pod node selector and tolerations in namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileSCCPrivileges(
		info.Username,
		req.Name,
		containercapabilties.NewContainerBuild(),
		checluster.IsContainerBuildCapabilitiesEnabled(),
	); err != nil {
		logrus.Errorf("Failed to reconcile the SCC privileges in namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	if err = r.reconcileSCCPrivileges(
		info.Username,
		req.Name,
		containercapabilties.NewContainerRun(),
		checluster.IsContainerRunCapabilitiesEnabled(),
	); err != nil {
		logrus.Errorf("Failed to reconcile the SCC privileges in namespace '%s': %v", req.Name, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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

	// Remove trusted-ca-certs ConfigMap from the target namespace to reduce the number of ConfigMaps
	// and avoid mounting the same certificates under different paths.
	// See cerificates#syncCheCABundleCerts
	trustedCACertsCMKey := client.ObjectKey{Name: prefixedName("trusted-ca-certs"), Namespace: targetNs}
	_, err := deploy.Delete(deployContext, trustedCACertsCMKey, &corev1.ConfigMap{})

	return err
}

func (r *CheUserNamespaceReconciler) reconcileUserSettings(
	deployContext *chetypes.DeployContext,
	targetNs string,
	checluster *chev2.CheCluster,
) error {
	cm2Delete := []string{
		prefixedName("editor-settings"),
		prefixedName("idle-settings"),
		prefixedName("proxy-settings"),
		checluster.Name + "-" + checluster.Namespace + "-proxy-settings", // legacy name
	}

	// delete previously created CMs
	for _, name := range cm2Delete {
		if _, err := deploy.Delete(
			deployContext,
			client.ObjectKey{Name: name, Namespace: targetNs},
			&corev1.ConfigMap{},
		); err != nil {
			return err
		}
	}

	name := prefixedName("user-settings")

	annotations := map[string]string{
		dwconstants.DevWorkspaceMountAsAnnotation: "env",
	}
	labels := defaults.AddStandardLabelsForComponent(checluster,
		userSettingsComponentLabelValue,
		map[string]string{
			dwconstants.DevWorkspaceMountLabel:          "true",
			dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
		})

	data := map[string]string{}

	// editor download urls
	if len(deployContext.CheCluster.Spec.DevEnvironments.EditorsDownloadUrls) > 0 {
		for _, editorDownloadUrl := range deployContext.CheCluster.Spec.DevEnvironments.EditorsDownloadUrls {
			editor := strings.ToUpper(editorDownloadUrl.Editor)
			editor = strings.ReplaceAll(editor, "-", "_")
			editor = strings.ReplaceAll(editor, "/", "_")
			data[fmt.Sprintf("EDITOR_DOWNLOAD_URL_%s", editor)] = editorDownloadUrl.Url
		}
	}

	// idling configuration
	if checluster.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling != nil {
		data["SECONDS_OF_DW_INACTIVITY_BEFORE_IDLING"] = strconv.FormatInt(int64(*checluster.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling), 10)
	}
	if checluster.Spec.DevEnvironments.SecondsOfRunBeforeIdling != nil {
		data["SECONDS_OF_DW_RUN_BEFORE_IDLING"] = strconv.FormatInt(int64(*checluster.Spec.DevEnvironments.SecondsOfRunBeforeIdling), 10)
	}

	// proxy settings
	if proxyConfig, err := che.GetProxyConfiguration(deployContext); err != nil {
		return err
	} else if proxyConfig != nil {
		if proxyConfig.HttpProxy != "" {
			data["HTTP_PROXY"] = proxyConfig.HttpProxy
			data["http_proxy"] = proxyConfig.HttpProxy
		}
		if proxyConfig.HttpsProxy != "" {
			data["HTTPS_PROXY"] = proxyConfig.HttpsProxy
			data["https_proxy"] = proxyConfig.HttpsProxy
		}
		if proxyConfig.NoProxy != "" {
			data["NO_PROXY"] = proxyConfig.NoProxy
			data["no_proxy"] = proxyConfig.NoProxy
		}
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   targetNs,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: data,
	}

	_, err := deploy.Sync(deployContext, cm, diffs.ConfigMapAllLabels)
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

	_, err := deploy.Sync(deployContext, &target, diffs.ConfigMapAllLabels)
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

func (r *CheUserNamespaceReconciler) reconcileSCCPrivileges(
	username string,
	targetNs string,
	containerCapability containercapabilties.ContainerCapability,
	isContainerCapabilitiesEnabled bool,
) error {
	if username == "" {
		return nil
	}

	if !isContainerCapabilitiesEnabled {
		return r.clientWrapper.DeleteByKeyIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: containerCapability.GetUserClusterRoleBindingName(), Namespace: targetNs},
			&rbacv1.RoleBinding{},
		)
	}

	rb := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      containerCapability.GetUserClusterRoleBindingName(),
			Namespace: targetNs,
			Labels:    map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     containerCapability.GetUserRoleName(),
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

	return r.clientWrapper.Sync(
		context.TODO(),
		rb,
		&k8sclient.SyncOptions{DiffOpts: diffs.RoleBinding},
	)
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
