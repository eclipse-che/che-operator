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

func (c *CertificatesReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if infrastructure.IsOpenShift() {
		if done, err := c.syncOpenShiftCABundleCertificates(cheCtx); !done {
			return reconcile.Result{}, false, err
		}
	} else {
		if done, err := c.syncKubernetesCABundleCertificates(cheCtx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if done, err := c.syncKubernetesRootCertificates(cheCtx); !done {
		return reconcile.Result{}, false, err
	}

	if done, err := c.syncGitTrustedCertificates(cheCtx); !done {
		return reconcile.Result{}, false, err
	}

	if cheCtx.IsSelfSignedCertificate {
		if done, err := c.syncSelfSignedCertificates(cheCtx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if cheCtx.Authentication.IssuerCA != "" {
		if done, err := c.syncOIDCIssuerCertificate(cheCtx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if done, err := c.syncCheCABundleCerts(cheCtx); !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *CertificatesReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	return true
}

func (c *CertificatesReconciler) syncOpenShiftCABundleCertificates(cheCtx *chetypes.CheContext) (bool, error) {
	openShiftCaBundleCMKey := types.NamespacedName{
		Namespace: cheCtx.CheCluster.Namespace,
		Name:      constants.DefaultCaBundleCertsCMName,
	}

	// Read ConfigMap with trusted CA certificates first.
	// It might contain custom certificates added there before the doc has been introduced
	// https://eclipse.dev/che/docs/stable/administration-guide/importing-untrusted-tls-certificates/
	openShiftCaBundleCM := &corev1.ConfigMap{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(context.TODO(), openShiftCaBundleCMKey, openShiftCaBundleCM)
	if err != nil {
		return false, fmt.Errorf("failed to read ConfigMap %s: %w", constants.DefaultCaBundleCertsCMName, err)
	}

	if !exists {
		openShiftCaBundleCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultCaBundleCertsCMName,
				Namespace: cheCtx.CheCluster.Namespace,
			},
		}
	}

	openShiftCaBundleCM.Labels = utils.GetMapOrDefault(openShiftCaBundleCM.Labels, map[string]string{})
	utils.AddMap(openShiftCaBundleCM.Labels, deploy.GetLabels(constants.CheCABundle))

	if cheCtx.CheCluster.IsDisableWorkspaceCaBundleMount() {
		// Remove annotation to stop OpenShift network operator from injecting certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		delete(openShiftCaBundleCM.Labels, constants.ConfigOpenShiftIOInjectTrustedCaBundle)
		delete(openShiftCaBundleCM.Annotations, constants.OpenShiftIOOwningComponent)

		// Remove key where OpenShift network operator injects certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		delete(openShiftCaBundleCM.Data, "ca-bundle.crt")

		// Add only custom certificates added by OpenShift Administrator
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/security_and_compliance/configuring-certificates#ca-bundle-understanding_updating-ca-bundle
		if cheCtx.Proxy.TrustedCAMapName != "" {
			trustedCACMKey := types.NamespacedName{
				Namespace: "openshift-config",
				Name:      cheCtx.Proxy.TrustedCAMapName,
			}

			trustedCACM := &corev1.ConfigMap{}
			if exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(context.TODO(), trustedCACMKey, trustedCACM); exists {
				openShiftCaBundleCM.Data = utils.GetMapOrDefault(openShiftCaBundleCM.Data, map[string]string{})
				openShiftCaBundleCM.Data["ca-bundle.crt"] = trustedCACM.Data["ca-bundle.crt"]
			} else if err != nil {
				return false, err
			}
		}
	} else {
		// Add annotation to allow OpenShift network operator inject certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		openShiftCaBundleCM.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle] = "true"
	}

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, openShiftCaBundleCM, cheCtx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	// adding `ConfigOpenShiftIOInjectTrustedCaBundle` label (even if deleted) ensures
	// that destination ConfigMap doesn't have it
	mandatoryLabelKeys := slices.Concat(deploy.GetLabelKeys(), []string{constants.ConfigOpenShiftIOInjectTrustedCaBundle})

	err = cheCtx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		openShiftCaBundleCM,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.ConfigMap(mandatoryLabelKeys, nil),
		})

	return err == nil, err
}

func (c *CertificatesReconciler) syncKubernetesCABundleCertificates(cheCtx *chetypes.CheContext) (bool, error) {
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
			Namespace:   cheCtx.CheCluster.Namespace,
			Labels:      deploy.GetLabels(constants.CheCABundle),
			Annotations: map[string]string{},
		},
		Data: map[string]string{kubernetesCABundleCertsFile: string(data)},
	}

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, kubernetesCaBundleCM, cheCtx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	err = cheCtx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		kubernetesCaBundleCM,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.ConfigMap(deploy.GetLabelKeys(), nil),
		},
	)

	return err == nil, err
}

// syncGitTrustedCertificates adds labels to git trusted certificates ConfigMap
// to include them into the final bundle
func (c *CertificatesReconciler) syncGitTrustedCertificates(cheCtx *chetypes.CheContext) (bool, error) {
	if cheCtx.CheCluster.Spec.DevEnvironments.TrustedCerts == nil || cheCtx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName == "" {
		return true, nil
	}

	gitTrustedCertsCM := &corev1.ConfigMap{}
	gitTrustedCertsKey := types.NamespacedName{
		Namespace: cheCtx.CheCluster.Namespace,
		Name:      cheCtx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName,
	}

	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(context.TODO(), gitTrustedCertsKey, gitTrustedCertsCM)
	if !exists {
		return err == nil, err
	}

	if gitTrustedCertsCM.Data[constants.GitSelfSignedCertsConfigMapCertKey] != "" {
		if gitTrustedCertsCM.GetLabels() == nil {
			gitTrustedCertsCM.Labels = map[string]string{}
		}

		// Add necessary labels to the ConfigMap
		gitTrustedCertsCM.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
		gitTrustedCertsCM.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

		// Don't need set SetControllerReference on this ConfigMap since it is created by admin

		err = cheCtx.ClusterAPI.ClientWrapper.Sync(
			context.TODO(),
			gitTrustedCertsCM,
			&k8sclient.SyncOptions{
				DiffOpts: diffs.ConfigMap(
					[]string{
						constants.KubernetesPartOfLabelKey,
						constants.KubernetesComponentLabelKey},
					nil,
				),
			},
		)

		return err == nil, err
	}

	return true, nil
}

// syncSelfSignedCertificates creates a ConfigMap with self-signed certificates and adds labels to it
// to include them into the final bundle
func (c *CertificatesReconciler) syncSelfSignedCertificates(cheCtx *chetypes.CheContext) (bool, error) {
	selfSignedCertSecret := &corev1.Secret{}
	selfSignedCertSecretKey := types.NamespacedName{
		Name:      constants.DefaultSelfSignedCertificateSecretName,
		Namespace: cheCtx.CheCluster.Namespace,
	}

	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(context.TODO(), selfSignedCertSecretKey, selfSignedCertSecret)
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
				Namespace:   cheCtx.CheCluster.Namespace,
				Labels:      deploy.GetLabels(constants.CheCABundle),
				Annotations: map[string]string{},
			},
			Data: map[string]string{"ca.crt": string(selfSignedCertSecret.Data["ca.crt"])},
		}

		if err := controllerutil.SetControllerReference(cheCtx.CheCluster, selfSignedCertCM, cheCtx.ClusterAPI.Scheme); err != nil {
			return false, err
		}

		err = cheCtx.ClusterAPI.ClientWrapper.Sync(
			context.TODO(),
			selfSignedCertCM,
			&k8sclient.SyncOptions{
				DiffOpts: diffs.ConfigMap(deploy.GetLabelKeys(), nil),
			})
	}

	return err == nil, err
}

// syncKubernetesRootCertificates adds labels to `kube-root-ca.crt` ConfigMap
// to include them into the final bundle
func (c *CertificatesReconciler) syncKubernetesRootCertificates(cheCtx *chetypes.CheContext) (bool, error) {
	kubeRootCertsCM := &corev1.ConfigMap{}
	kubeRootCertsCMKey := types.NamespacedName{
		Name:      kubernetesRootCACertsCMName,
		Namespace: cheCtx.CheCluster.Namespace,
	}

	exists, err := cheCtx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(context.TODO(), kubeRootCertsCMKey, kubeRootCertsCM)
	if !exists {
		return err == nil, err
	}

	if kubeRootCertsCM.GetLabels() == nil {
		kubeRootCertsCM.SetLabels(map[string]string{})
	}

	// Add necessary labels to the ConfigMap
	kubeRootCertsCM.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
	kubeRootCertsCM.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

	err = cheCtx.ClusterAPI.NonCachingClientWrapper.Sync(
		context.TODO(),
		kubeRootCertsCM,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.ConfigMap(
				[]string{
					constants.KubernetesPartOfLabelKey,
					constants.KubernetesComponentLabelKey},
				nil,
			),
		})

	return err == nil, err
}

func (c *CertificatesReconciler) syncOIDCIssuerCertificate(cheCtx *chetypes.CheContext) (bool, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      OIDCIssuerCACMName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.CheCABundle),
		},
		Data: map[string]string{
			"ca-bundle.crt": cheCtx.Authentication.IssuerCA,
		},
	}

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, cm, cheCtx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	err := cheCtx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		cm,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.ConfigMap(deploy.GetLabelKeys(), nil),
		},
	)
	return err == nil, err
}

// syncCheCABundleCerts merges all trusted CA certificates into a single ConfigMap `ca-certs-merged`,
// adds labels and annotations to mount it into dev workspaces.
func (c *CertificatesReconciler) syncCheCABundleCerts(cheCtx *chetypes.CheContext) (bool, error) {
	// Get all ConfigMaps with trusted CA certificates
	cheCABundlesCMs, err := GetCheCABundles(cheCtx.ClusterAPI.Client, cheCtx.CheCluster.GetNamespace())
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
			if dataKey == constants.GitSelfSignedCertsConfigMapGitHostKey && isGitTrustedCertsConfigMap(cheCtx, &cm) {
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
			Namespace:   cheCtx.CheCluster.Namespace,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}

	if len(strings.TrimSpace(cheCABundlesContent)) != 0 {
		mergedCABundlesCM.Data[kubernetesCABundleCertsFile] = cheCABundlesContent
	}

	if !cheCtx.CheCluster.IsDisableWorkspaceCaBundleMount() {
		// Mount the CA bundle into /etc/pki/ca-trust/extracted/pem
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "subpath"
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = kubernetesCABundleCertsDir
	} else {
		// Default behavior is to mount the CA bundle into /public-certs
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "file"
		mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = constants.PublicCertsDir
	}
	mergedCABundlesCM.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation] = "0444"

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, mergedCABundlesCM, cheCtx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	err = cheCtx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		mergedCABundlesCM,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.ConfigMap(deploy.GetLabelsAndAnnotations(mergedCABundlesCM)),
		},
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

func isGitTrustedCertsConfigMap(cheCtx *chetypes.CheContext, cm *corev1.ConfigMap) bool {
	if cm.Name == constants.DefaultGitSelfSignedCertsConfigMapName {
		return true
	}

	if cheCtx.CheCluster.Spec.DevEnvironments.TrustedCerts != nil &&
		cm.Name == cheCtx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName {
		return true
	}

	return false
}
