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
	"sort"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"

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
		delete(openShiftCaBundleCM.ObjectMeta.Labels, constants.ConfigOpenShiftIOInjectTrustedCaBundle)
		delete(openShiftCaBundleCM.ObjectMeta.Annotations, constants.OpenShiftIOOwningComponent)

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

		return deploy.Sync(
			ctx,
			openShiftCaBundleCM,
			diffs.ConfigMap(append(deploy.DefaultsLabelKeys, constants.ConfigOpenShiftIOInjectTrustedCaBundle), nil),
		)
	} else {
		// Add annotation to allow OpenShift network operator inject certificates
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/networking/configuring-a-custom-pki#certificate-injection-using-operators_configuring-a-custom-pki
		openShiftCaBundleCM.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle] = "true"

		// Ignore Data field to allow OpenShift network operator inject certificates into CM
		// and avoid endless reconciliation loop
		return deploy.Sync(
			ctx,
			openShiftCaBundleCM,
			diffs.ConfigMap(append(deploy.DefaultsLabelKeys, constants.ConfigOpenShiftIOInjectTrustedCaBundle), nil),
		)
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

	return deploy.Sync(ctx, kubernetesCaBundleCM, diffs.ConfigMapAllLabels)
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

		if gitTrustedCertsCM.GetLabels() == nil {
			gitTrustedCertsCM.Labels = map[string]string{}
		}

		// Add necessary labels to the ConfigMap
		gitTrustedCertsCM.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
		gitTrustedCertsCM.Labels[constants.KubernetesComponentLabelKey] = constants.CheCABundle

		return deploy.Sync(
			ctx,
			gitTrustedCertsCM,
			diffs.ConfigMap([]string{constants.KubernetesPartOfLabelKey, constants.KubernetesComponentLabelKey}, nil),
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

		return deploy.Sync(ctx, selfSignedCertCM, diffs.ConfigMapAllLabels)
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
		diffs.ConfigMap([]string{constants.KubernetesPartOfLabelKey, constants.KubernetesComponentLabelKey}, nil),
	)
}

// syncCheCABundleCerts merges all trusted CA certificates into a single ConfigMap `ca-certs-merged`,
// adds labels and annotations to mount it into dev workspaces.
func (c *CertificatesReconciler) syncCheCABundleCerts(ctx *chetypes.DeployContext) (bool, error) {
	// Get all ConfigMaps with trusted CA certificates
	cheCABundlesCMs, err := GetCheCABundles(ctx.ClusterAPI.Client, ctx.CheCluster.GetNamespace())
	if err != nil {
		return false, err
	}

	// Sort configmaps by name, always have the same order and content
	// to avoid endless reconcile loop
	sort.Slice(cheCABundlesCMs, func(i, j int) bool {
		return strings.Compare(cheCABundlesCMs[i].Name, cheCABundlesCMs[j].Name) < 0
	})

	// Calculated revisions and content
	cheCABundlesContent := ""
	for _, cm := range cheCABundlesCMs {
		for dataKey, dataValue := range cm.Data {
			cheCABundlesContent += fmt.Sprintf(
				"# ConfigMap: %s,  Key: %s\n%s\n\n",
				cm.Name,
				dataKey,
				dataValue,
			)
		}
	}

	// Sync a new ConfigMap with all trusted CA certificates
	mergedCABundlesCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CheMergedCABundleCertsCMName,
			Namespace: ctx.CheCluster.Namespace,
			// Mark ConfigMap as workspace config (will be mounted in all users' containers)
			Labels:      deploy.GetLabels(constants.WorkspacesConfig),
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}

	if len(strings.TrimSpace(cheCABundlesContent)) != 0 {
		mergedCABundlesCM.Data[kubernetesCABundleCertsFile] = cheCABundlesContent
	}

	if !ctx.CheCluster.IsDisableWorkspaceCaBundleMount() {
		// Mount the CA bundle into /etc/pki/ca-trust/extracted/pem
		mergedCABundlesCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "subpath"
		mergedCABundlesCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = kubernetesCABundleCertsDir
	} else {
		// Default behavior is to mount the CA bundle into /public-certs
		mergedCABundlesCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "file"
		mergedCABundlesCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = constants.PublicCertsDir
	}
	mergedCABundlesCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation] = "0444"

	return deploy.Sync(
		ctx,
		mergedCABundlesCM,
		diffs.ConfigMap(
			deploy.DefaultsLabelKeys,
			[]string{
				dwconstants.DevWorkspaceMountAsAnnotation,
				dwconstants.DevWorkspaceMountPathAnnotation,
				dwconstants.DevWorkspaceMountAccessModeAnnotation,
			},
		),
	)
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
