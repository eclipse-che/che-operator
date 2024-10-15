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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// OpenShift annotation to inject trusted CA bundle
	injectTrustedCaBundle       = "config.openshift.io/inject-trusted-cabundle"
	kubernetesRootCACertsCMName = "kube-root-ca.crt"
	kubernetesCABundleCertsDir  = "/etc/pki/ca-trust/extracted/pem"
	kubernetesCABundleCertsFile = "tls-ca-bundle.pem"

	// The ConfigMap name for merged CA bundle certificates
	CheMergedCABundleCertsCMName = "ca-certs-merged"

	// Annotation holds revisions of included config maps
	// in the format: name1#ver1 name2=ver2
	cheCABundleIncludedCMRevisions = "che.eclipse.org/included-configmaps"
	entrySplitter                  = " "
	keyValueSplitter               = "#"
)

type CertificatesReconciler struct {
	deploy.Reconcilable
}

func NewCertificatesReconciler() *CertificatesReconciler {
	return &CertificatesReconciler{}
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

	if done, err := c.syncCheCABundleCerts(ctx); !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *CertificatesReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (c *CertificatesReconciler) syncOpenShiftCABundleCertificates(ctx *chetypes.DeployContext) (bool, error) {
	openShiftCaBundleCM := deploy.GetConfigMapSpec(
		ctx,
		constants.DefaultCaBundleCertsCMName,
		map[string]string{},
		defaults.GetCheFlavor(),
	)
	openShiftCaBundleCM.ObjectMeta.Labels[injectTrustedCaBundle] = "true"
	openShiftCaBundleCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

	return deploy.Sync(
		ctx,
		openShiftCaBundleCM,
		cmp.Options{
			cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
			cmpopts.IgnoreFields(corev1.ConfigMap{}, "Data"),
			cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
				return x.Labels[injectTrustedCaBundle] == y.Labels[injectTrustedCaBundle] &&
					x.Labels[constants.KubernetesComponentLabelKey] == y.Labels[constants.KubernetesComponentLabelKey]
			}),
		},
	)
}

func (c *CertificatesReconciler) syncKubernetesCABundleCertificates(ctx *chetypes.DeployContext) (bool, error) {
	data, err := os.ReadFile(kubernetesCABundleCertsDir + string(os.PathSeparator) + kubernetesCABundleCertsFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}

		return false, err
	}

	kubernetesCaBundleCM := deploy.GetConfigMapSpec(
		ctx,
		constants.DefaultCaBundleCertsCMName,
		map[string]string{"ca.crt": string(data)},
		constants.CheCABundle,
	)

	return deploy.Sync(ctx, kubernetesCaBundleCM, deploy.ConfigMapDiffOpts)
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

	if gitTrustedCertsCM.Data["ca.crt"] != "" {
		gitTrustedCertsCM.TypeMeta = metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}

		// Add necessary labels to the ConfigMap
		gitTrustedCertsCM.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
		gitTrustedCertsCM.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

		return deploy.Sync(
			ctx,
			gitTrustedCertsCM,
			cmp.Options{
				cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
				cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
					return x.Labels[constants.KubernetesPartOfLabelKey] == y.Labels[constants.KubernetesPartOfLabelKey] &&
						x.Labels[constants.KubernetesComponentLabelKey] == y.Labels[constants.KubernetesComponentLabelKey]
				}),
			})
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
		selfSignedCertCM := deploy.GetConfigMapSpec(
			ctx,
			constants.DefaultSelfSignedCertificateSecretName,
			map[string]string{
				"ca.crt": string(selfSignedCertSecret.Data["ca.crt"]),
			},
			constants.CheCABundle)

		return deploy.Sync(ctx, selfSignedCertCM, deploy.ConfigMapDiffOpts)
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

	return deploy.SyncForClient(
		client,
		ctx,
		kubeRootCertsCM,
		cmp.Options{
			cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
			cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
				return x.Labels[constants.KubernetesPartOfLabelKey] == y.Labels[constants.KubernetesPartOfLabelKey] &&
					x.Labels[constants.KubernetesComponentLabelKey] == y.Labels[constants.KubernetesComponentLabelKey]
			}),
		})
}

// syncCheCABundleCerts merges all trusted CA certificates into a single ConfigMap `ca-certs-merged`,
// adds labels and annotations to mount it into dev workspaces.
func (c *CertificatesReconciler) syncCheCABundleCerts(ctx *chetypes.DeployContext) (bool, error) {
	// Get all ConfigMaps with trusted CA certificates
	cheCABundlesCMs, err := GetCheCABundles(ctx.ClusterAPI.Client, ctx.CheCluster.GetNamespace())
	if err != nil {
		return false, err
	}

	// Calculated expected revisions and content
	cheCABundlesExpectedContent := ""
	cheCABundleExpectedRevisions := make(map[string]string)
	cheCABundlesExpectedRevisionsAsString := ""
	for _, cm := range cheCABundlesCMs {
		cheCABundleExpectedRevisions[cm.Name] = cm.ResourceVersion
		cheCABundlesExpectedRevisionsAsString +=
			cm.ObjectMeta.Name + keyValueSplitter + cm.ObjectMeta.ResourceVersion + entrySplitter

		for dataKey, dataValue := range cm.Data {
			cheCABundlesExpectedContent += fmt.Sprintf(
				"# ConfigMap: %s,  Key: %s\n%s\n\n",
				cm.Name,
				dataKey,
				dataValue,
			)
		}
	}

	// Calculated actual revisions
	mergedCABundlesCM := &corev1.ConfigMap{}
	mergedCABundlesCMKey := types.NamespacedName{
		Name:      CheMergedCABundleCertsCMName,
		Namespace: ctx.CheCluster.Namespace,
	}

	exists, err := deploy.Get(ctx, mergedCABundlesCMKey, mergedCABundlesCM)
	if err != nil {
		return false, err
	}

	if exists {
		cheCABundleCMActualRevisions := make(map[string]string)
		if mergedCABundlesCM.GetAnnotations() != nil {
			if revs, ok := mergedCABundlesCM.ObjectMeta.Annotations[cheCABundleIncludedCMRevisions]; ok {
				for _, rev := range strings.Split(revs, entrySplitter) {
					item := strings.Split(rev, keyValueSplitter)
					if len(item) == 2 {
						cheCABundleCMActualRevisions[item[0]] = item[1]
					}
				}
			}
		}

		// Compare actual and expected revisions to check if we need to update the ConfigMap
		if reflect.DeepEqual(cheCABundleExpectedRevisions, cheCABundleCMActualRevisions) {
			return true, nil
		}
	}

	// Sync a new ConfigMap with all trusted CA certificates
	mergedCABundlesCM = deploy.GetConfigMapSpec(
		ctx,
		CheMergedCABundleCertsCMName,
		map[string]string{"ca.crt": cheCABundlesExpectedContent},
		defaults.GetCheFlavor(),
	)

	// Add annotations with included config maps revisions
	mergedCABundlesCM.ObjectMeta.Annotations[cheCABundleIncludedCMRevisions] = cheCABundlesExpectedRevisionsAsString

	if ctx.CheCluster.Spec.DevEnvironments.TrustedCerts == nil ||
		(ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.DisableMountingCaBundleIntoDevWorkspace != nil &&
			!*ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.DisableMountingCaBundleIntoDevWorkspace) {

		mountPaths := constants.DefaultCaBundleMountPaths
		if ctx.CheCluster.Spec.DevEnvironments.TrustedCerts != nil &&
			len(ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.CaBundleMountPaths) > 0 {
			mountPaths = ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.CaBundleMountPaths
		}

		// Mount CA bundle into dev workspaces
		// TODO rework to create a dedicated CM for every mount path
		for _, mountPath := range mountPaths {
			mountDir, mountFileName := filepath.Split(mountPath)

			// Mark ConfigMap as workspace config (will be mounted in all workspace pods)
			mergedCABundlesCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey] = constants.WorkspacesConfig

			// Add annotations to mount certificates in desired path
			mergedCABundlesCM.ObjectMeta.Annotations["controller.devfile.io/mount-as"] = "subpath"
			mergedCABundlesCM.ObjectMeta.Annotations["controller.devfile.io/mount-path"] = mountDir

			mergedCABundlesCM.Data = map[string]string{mountFileName: cheCABundlesExpectedContent}
		}
	}

	return deploy.Sync(ctx, mergedCABundlesCM, deploy.ConfigMapDiffOpts)
}
