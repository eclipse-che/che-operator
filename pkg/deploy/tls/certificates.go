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

package tls

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/eclipse-che/che-operator/pkg/common/utils"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	kubernetesRootCACertsCMName = "kube-root-ca.crt"
	kubernetesCABundleCertsDir  = "/etc/pki/ca-trust/extracted/pem"
	kubernetesCABundleCertsFile = "tls-ca-bundle.pem"

	// The ConfigMap name for merged CA bundle certificates
	CheMergedCABundleCertsCMName = "ca-certs-merged"
	OIDCIssuerCACMName           = "oidc-issuer-ca"
)

type CertificatesReconciler struct {
	reconciler.Reconcilable
	readKubernetesCaBundle func() ([]byte, error)
}

func NewCertificatesReconciler() *CertificatesReconciler {
	return &CertificatesReconciler{
		readKubernetesCaBundle: readKubernetesCaBundle,
	}
}

func (c *CertificatesReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if infrastructure.IsOpenShift() {
		if done, err := c.syncOpenShiftCABundleCertificates(ctx); !done {
			return reconcile.Result{}, false, err
		}
	} else {
		if done, err := c.syncKubernetesCABundleCertificates(ctx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if done, err := c.syncKubernetesRootCertificates(ctx); !done {
		return reconcile.Result{}, false, err
	}

	if done, err := c.syncGitTrustedCertificates(ctx); !done {
		return reconcile.Result{}, false, err
	}

	if ctx.IsSelfSignedCertificate {
		if done, err := c.syncSelfSignedCertificates(ctx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if ctx.Authentication.IssuerCA != "" {
		if done, err := c.syncOIDCIssuerCertificate(ctx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if done, err := c.syncCheCABundleCerts(ctx); !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *CertificatesReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (c *CertificatesReconciler) syncOpenShiftCABundleCertificates(ctx *chetypes.DeployContext) (bool, error) {
	openShiftCaBundleCMKey := types.NamespacedName{
		Namespace: ctx.CheCluster.Namespace,
		Name:      constants.DefaultCaBundleCertsCMName,
	}

	// Read ConfigMap with trusted CA certificates first.
	// It might contain custom certificates added there before the doc has been introduced
	// https://eclipse.dev/che/docs/stable/administration-guide/importing-untrusted-tls-certificates/
	openShiftCaBundleCM := &corev1.ConfigMap{}
	exists, err := deploy.Get(ctx, openShiftCaBundleCMKey, openShiftCaBundleCM)
	if err != nil {
		return false, err
	}

	if !exists {
		openShiftCaBundleCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultCaBundleCertsCMName,
				Namespace: ctx.CheCluster.Namespace,
			},
		}
	}

	// Ensure TypeMeta to avoid "cause: no version "" has been registered in scheme" error
	openShiftCaBundleCM.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}

	openShiftCaBundleCM.Labels = utils.GetMapOrDefault(openShiftCaBundleCM.Labels, map[string]string{})
	utils.AddMap(openShiftCaBundleCM.Labels, deploy.GetLabels(constants.CheCABundle))

	if ctx.CheCluster.IsDisableWorkspaceCaBundleMount() {
		// Remove annotation to stop OpenShift network operator from injecting certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		delete(openShiftCaBundleCM.Labels, constants.ConfigOpenShiftIOInjectTrustedCaBundle)
		delete(openShiftCaBundleCM.Annotations, constants.OpenShiftIOOwningComponent)

		// Remove key where OpenShift network operator injects certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		delete(openShiftCaBundleCM.Data, "ca-bundle.crt")

		// Add only custom certificates added by OpenShift Administrator
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/security_and_compliance/configuring-certificates#ca-bundle-understanding_updating-ca-bundle
		if ctx.Proxy.TrustedCAMapName != "" {
			trustedCACMKey := types.NamespacedName{
				Namespace: "openshift-config",
				Name:      ctx.Proxy.TrustedCAMapName,
			}

			trustedCACM := &corev1.ConfigMap{}
			if exists, err := deploy.Get(ctx, trustedCACMKey, trustedCACM); exists {
				openShiftCaBundleCM.Data = utils.GetMapOrDefault(openShiftCaBundleCM.Data, map[string]string{})
				openShiftCaBundleCM.Data["ca-bundle.crt"] = trustedCACM.Data["ca-bundle.crt"]
			} else if err != nil {
				return false, err
			}
		}

		labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(openShiftCaBundleCM)

		// add removed label to ensure a new object won't have it
		labelKeys = append(labelKeys, constants.ConfigOpenShiftIOInjectTrustedCaBundle)

		return deploy.Sync(ctx, openShiftCaBundleCM, diffs.ConfigMapWithMetadata(labelKeys, annotationKeys))
	} else {
		// Add annotation to allow OpenShift network operator inject certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		openShiftCaBundleCM.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle] = "true"

		// Ignore Data field to allow OpenShift network operator inject certificates into CM
		// and avoid endless reconciliation loop
		labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(openShiftCaBundleCM)
		return deploy.Sync(ctx, openShiftCaBundleCM, diffs.ConfigMapWithMetadata(labelKeys, annotationKeys))
	}
}

func (c *CertificatesReconciler) syncKubernetesCABundleCertificates(ctx *chetypes.DeployContext) (bool, error) {
	data, err := c.readKubernetesCaBundle()
	if err != nil {
		return false, err
	}

	kubernetesCaBundleCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.DefaultCaBundleCertsCMName,
			Namespace:   ctx.CheCluster.Namespace,
			Labels:      deploy.GetLabels(constants.CheCABundle),
			Annotations: map[string]string{},
		},
		Data: map[string]string{kubernetesCABundleCertsFile: string(data)},
	}

	labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(kubernetesCaBundleCM)
	return deploy.Sync(ctx, kubernetesCaBundleCM, diffs.ConfigMapWithMetadata(labelKeys, annotationKeys))
}

// syncGitTrustedCertificates adds labels to git trusted certificates ConfigMap
// to include them into the final bundle
func (c *CertificatesReconciler) syncGitTrustedCertificates(ctx *chetypes.DeployContext) (bool, error) {
	if ctx.CheCluster.Spec.DevEnvironments.TrustedCerts == nil || ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName == "" {
		return true, nil
	}

	gitTrustedCertsCM := &corev1.ConfigMap{}
	gitTrustedCertsKey := types.NamespacedName{
		Namespace: ctx.CheCluster.Namespace,
		Name:      ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName,
	}

	exists, err := deploy.Get(ctx, gitTrustedCertsKey, gitTrustedCertsCM)
	if !exists {
		return err == nil, err
	}

	if gitTrustedCertsCM.Data[constants.GitSelfSignedCertsConfigMapCertKey] != "" {
		gitTrustedCertsCM.TypeMeta = metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}

		if gitTrustedCertsCM.GetLabels() == nil {
			gitTrustedCertsCM.Labels = map[string]string{}
		}

		// Add necessary labels to the ConfigMap
		gitTrustedCertsCM.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
		gitTrustedCertsCM.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

		labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(gitTrustedCertsCM)
		return deploy.Sync(
			ctx,
			gitTrustedCertsCM,
			diffs.ConfigMapWithMetadata(labelKeys, annotationKeys),
		)
	}

	return true, nil
}

// syncSelfSignedCertificates creates a ConfigMap with self-signed certificates and adds labels to it
// to include them into the final bundle
func (c *CertificatesReconciler) syncSelfSignedCertificates(ctx *chetypes.DeployContext) (bool, error) {
	selfSignedCertSecret := &corev1.Secret{}
	selfSignedCertSecretKey := types.NamespacedName{
		Name:      constants.DefaultSelfSignedCertificateSecretName,
		Namespace: ctx.CheCluster.Namespace,
	}

	exists, err := deploy.Get(ctx, selfSignedCertSecretKey, selfSignedCertSecret)
	if !exists {
		return err == nil, err
	}

	if len(selfSignedCertSecret.Data["ca.crt"]) > 0 {
		selfSignedCertCM := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        constants.DefaultSelfSignedCertificateSecretName,
				Namespace:   ctx.CheCluster.Namespace,
				Labels:      deploy.GetLabels(constants.CheCABundle),
				Annotations: map[string]string{},
			},
			Data: map[string]string{"ca.crt": string(selfSignedCertSecret.Data["ca.crt"])},
		}

		labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(selfSignedCertCM)
		return deploy.Sync(ctx, selfSignedCertCM, diffs.ConfigMapWithMetadata(labelKeys, annotationKeys))
	}

	return true, nil
}

// syncKubernetesRootCertificates adds labels to `kube-root-ca.crt` ConfigMap
// to include them into the final bundle
func (c *CertificatesReconciler) syncKubernetesRootCertificates(ctx *chetypes.DeployContext) (bool, error) {
	client := ctx.ClusterAPI.NonCachingClient
	kubeRootCertsCM := &corev1.ConfigMap{}
	kubeRootCertsCMKey := types.NamespacedName{
		Name:      kubernetesRootCACertsCMName,
		Namespace: ctx.CheCluster.Namespace,
	}

	exists, err := deploy.GetForClient(client, kubeRootCertsCMKey, kubeRootCertsCM)
	if !exists {
		return err == nil, err
	}

	if kubeRootCertsCM.GetLabels() == nil {
		kubeRootCertsCM.SetLabels(map[string]string{})
	}

	// Set TypeMeta to avoid "cause: no version "" has been registered in scheme" error
	kubeRootCertsCM.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}

	// Add necessary labels to the ConfigMap
	kubeRootCertsCM.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
	kubeRootCertsCM.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

	labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(kubeRootCertsCM)
	return deploy.SyncForClient(
		client,
		ctx,
		kubeRootCertsCM,
		diffs.ConfigMapWithMetadata(labelKeys, annotationKeys),
	)
}

func (c *CertificatesReconciler) syncOIDCIssuerCertificate(ctx *chetypes.DeployContext) (bool, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      OIDCIssuerCACMName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.CheCABundle),
		},
		Data: map[string]string{
			"ca-bundle.crt": ctx.Authentication.IssuerCA,
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, cm, ctx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(cm)
	err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		cm,
		&k8sclient.SyncOptions{DiffOpts: diffs.ConfigMapWithMetadata(labelKeys, annotationKeys)},
	)
	return err == nil, err
}

// syncCheCABundleCerts merges all trusted CA certificates into a single ConfigMap `ca-certs-merged`,
// adds labels and annotations to mount it into dev workspaces.
func (c *CertificatesReconciler) syncCheCABundleCerts(ctx *chetypes.DeployContext) (bool, error) {
	// Get all ConfigMaps with trusted CA certificates
	cheCABundlesCMs, err := GetCheCABundles(ctx.ClusterAPI.Client, ctx.CheCluster.GetNamespace())
	if err != nil {
		return false, err
	}

	// Sort ConfigMaps by name and their data keys alphabetically to ensure
	// deterministic ordering. This prevents spurious reconcile loops that occur
	// when Go's random map iteration produces different output each time.
	sort.Slice(cheCABundlesCMs, func(i, j int) bool {
		return strings.Compare(cheCABundlesCMs[i].Name, cheCABundlesCMs[j].Name) < 0
	})

	cheCABundlesContent := ""
	for _, cm := range cheCABundlesCMs {
		// Sort keys to produce deterministic output and avoid endless reconcile loop
		dataKeys := slices.Collect(maps.Keys(cm.Data))
		sort.Strings(dataKeys)

		for _, dataKey := range dataKeys {
			// Skip the "githost" key from the git trusted certs ConfigMap:
			// it contains a hostname, not a certificate, and should not be included in the CA bundle.
			if dataKey == constants.GitSelfSignedCertsConfigMapGitHostKey && isGitTrustedCertsConfigMap(ctx, &cm) {
				continue
			}

			cheCABundlesContent += printCert(&cm, dataKey)
		}
	}

	// Mark ConfigMap as workspace config (will be mounted in all users' containers)
	labels := deploy.GetLabels(constants.WorkspacesConfig)

	// Mark as `controller.devfile.io/watch-configmap=true` to allow DWO read custom certificates
	labels[dwconstants.DevWorkspaceWatchConfigMapLabel] = "true"

	// Sync a new ConfigMap with all trusted CA certificates
	mergedCABundlesCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        CheMergedCABundleCertsCMName,
			Namespace:   ctx.CheCluster.Namespace,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}

	if len(strings.TrimSpace(cheCABundlesContent)) != 0 {
		mergedCABundlesCM.Data[kubernetesCABundleCertsFile] = cheCABundlesContent
	}

	if !ctx.CheCluster.IsDisableWorkspaceCaBundleMount() {
		// Mount the CA bundle into /etc/pki/ca-trust/extracted/pem
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "subpath"
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = kubernetesCABundleCertsDir
	} else {
		// Default behavior is to mount the CA bundle into /public-certs
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "file"
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = constants.PublicCertsDir
	}
	mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation] = "0444"

	if err := controllerutil.SetControllerReference(ctx.CheCluster, mergedCABundlesCM, ctx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	labelKeys, annotationKeys := diffs.GetLabelsAndAnnotations(mergedCABundlesCM)
	err = ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		mergedCABundlesCM,
		&k8sclient.SyncOptions{DiffOpts: diffs.ConfigMapWithMetadata(labelKeys, annotationKeys)},
	)
	return err == nil, err
}

func readKubernetesCaBundle() ([]byte, error) {
	data, err := os.ReadFile(kubernetesCABundleCertsDir + string(os.PathSeparator) + kubernetesCABundleCertsFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	return data, nil
}

// printCert formats a single certificate entry with its ConfigMap name and key as a header comment.
func printCert(cm *corev1.ConfigMap, key string) string {
	return fmt.Sprintf(
		"# ConfigMap: %s,  Key: %s\n%s\n\n",
		cm.Name,
		key,
		cm.Data[key],
	)
}

func isGitTrustedCertsConfigMap(ctx *chetypes.DeployContext, cm *corev1.ConfigMap) bool {
	if cm.Name == constants.DefaultGitSelfSignedCertsConfigMapName {
		return true
	}

	if ctx.CheCluster.Spec.DevEnvironments.TrustedCerts != nil &&
		cm.Name == ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName {
		return true
	}

	return false
}
