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
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	injector = "config.openshift.io/inject-trusted-cabundle"
	// CheCACertsConfigMapLabelKey is the label value which marks config map with additional CA certificates
	CheCACertsConfigMapLabelValue = "ca-bundle"
	// CheAllCACertsConfigMapName is the name of config map which contains all additional trusted by Che TLS CA certificates
	CheAllCACertsConfigMapName = "ca-certs-merged"
	// CheMergedCAConfigMapRevisionsAnnotationKey is annotation name which holds versions of included config maps in format: cm-name1=ver1,cm-name2=ver2
	CheMergedCAConfigMapRevisionsAnnotationKey = "che.eclipse.org/included-configmaps"

	KubernetesRootCertificateConfigMapName = "kube-root-ca.crt"

	// Local constants
	// labelEqualSign constant is used as a replacement for '=' symbol in labels because '=' is not allowed there
	labelEqualSign = "-"
	// labelCommaSign constant is used as a replacement for ',' symbol in labels because ',' is not allowed there
	labelCommaSign = "."
)

type CertificatesReconciler struct {
	deploy.Reconcilable
}

func NewCertificatesReconciler() *CertificatesReconciler {
	return &CertificatesReconciler{}
}

func (c *CertificatesReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.Proxy.TrustedCAMapName != "" {
		if done, err := c.syncTrustStoreConfigMapToCluster(ctx); !done {
			return reconcile.Result{}, false, err
		}
	}

	if done, err := c.syncKubernetesRootCertificates(ctx); !done {
		return reconcile.Result{}, false, err
	}

	if done, err := c.syncAdditionalCACertsConfigMapToCluster(ctx); !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *CertificatesReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (c *CertificatesReconciler) syncTrustStoreConfigMapToCluster(ctx *chetypes.DeployContext) (bool, error) {
	configMapSpec := deploy.GetConfigMapSpec(ctx, constants.DefaultServerTrustStoreConfigMapName, map[string]string{}, defaults.GetCheFlavor())

	// OpenShift will automatically injects all certs into the configmap
	configMapSpec.ObjectMeta.Labels[injector] = "true"
	configMapSpec.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
	configMapSpec.ObjectMeta.Labels[constants.KubernetesComponentLabelKey] = CheCACertsConfigMapLabelValue

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(ctx, constants.DefaultServerTrustStoreConfigMapName, actual)
	if err != nil {
		return false, err
	}

	if !exists {
		// We have to create an empty config map with the specific labels
		done, err := deploy.Create(ctx, configMapSpec)
		return done, err
	}

	if actual.ObjectMeta.Labels[injector] != "true" ||
		actual.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey] != constants.CheEclipseOrg ||
		actual.ObjectMeta.Labels[constants.KubernetesComponentLabelKey] != CheCACertsConfigMapLabelValue {

		actual.ObjectMeta.Labels[injector] = "true"
		actual.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
		actual.ObjectMeta.Labels[constants.KubernetesComponentLabelKey] = CheCACertsConfigMapLabelValue

		logrus.Infof("Updating existed object: %s, name: %s", configMapSpec.Kind, configMapSpec.Name)
		if err := ctx.ClusterAPI.Client.Update(context.TODO(), actual); err != nil {
			return false, err
		}
	}

	return true, nil
}

// syncAdditionalCACertsConfigMapToCluster adds labels to ConfigMap `kube-root-ca.crt` to propagate
// Kubernetes root certificates to Che components. It is needed to use NonCachingClient because the map
// initially is not in the cache.
func (c *CertificatesReconciler) syncKubernetesRootCertificates(ctx *chetypes.DeployContext) (bool, error) {
	kubeRootCertsConfigMap := &corev1.ConfigMap{}
	if err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      KubernetesRootCertificateConfigMapName,
			Namespace: ctx.CheCluster.Namespace,
		},
		kubeRootCertsConfigMap); err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		} else {
			return false, err
		}
	}

	if kubeRootCertsConfigMap.GetLabels() == nil {
		kubeRootCertsConfigMap.SetLabels(map[string]string{})
	}

	kubeRootCertsConfigMap.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
	kubeRootCertsConfigMap.Labels[constants.KubernetesComponentLabelKey] = CheCACertsConfigMapLabelValue

	// Set TypeMeta to avoid "cause: no version "" has been registered in scheme" error
	kubeRootCertsConfigMap.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}
	return deploy.SyncWithClient(ctx.ClusterAPI.NonCachingClient, ctx, kubeRootCertsConfigMap, deploy.ConfigMapDiffOpts)
}

func (c *CertificatesReconciler) syncAdditionalCACertsConfigMapToCluster(ctx *chetypes.DeployContext) (bool, error) {
	// Get all source config maps, if any
	caConfigMaps, err := GetCACertsConfigMaps(ctx.ClusterAPI.Client, ctx.CheCluster.GetNamespace())
	if err != nil {
		return false, err
	}

	mergedCAConfigMap := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: CheAllCACertsConfigMapName}, mergedCAConfigMap)
	if err == nil {
		// Merged config map exists. Check if it is up to date.
		caConfigMapsCurrentRevisions := make(map[string]string)
		for _, cm := range caConfigMaps {
			caConfigMapsCurrentRevisions[cm.Name] = cm.ResourceVersion
		}

		caConfigMapsCachedRevisions := make(map[string]string)
		if mergedCAConfigMap.ObjectMeta.Annotations != nil {
			if revisions, exists := mergedCAConfigMap.ObjectMeta.Annotations[CheMergedCAConfigMapRevisionsAnnotationKey]; exists {
				for _, cmNameRevision := range strings.Split(revisions, labelCommaSign) {
					nameRevision := strings.Split(cmNameRevision, labelEqualSign)
					if len(nameRevision) != 2 {
						// The label value is invalid, recreate merged config map
						break
					}
					caConfigMapsCachedRevisions[nameRevision[0]] = nameRevision[1]
				}
			}
		}

		if reflect.DeepEqual(caConfigMapsCurrentRevisions, caConfigMapsCachedRevisions) {
			// Existing merged config map is up to date, do nothing
			return true, nil
		}
	} else {
		if !errors.IsNotFound(err) {
			return false, err
		}
		// Merged config map doesn't exist. Create it.
	}

	// Merged config map is out of date or doesn't exist
	// Merge all config maps into single one to mount inside Che components and workspaces
	data := make(map[string]string)
	revisions := ""
	for _, cm := range caConfigMaps {
		// Copy data
		for key, dataRecord := range cm.Data {
			data[cm.ObjectMeta.Name+"."+key] = dataRecord
		}

		// Save source config map revision
		if revisions != "" {
			revisions += labelCommaSign
		}
		revisions += cm.ObjectMeta.Name + labelEqualSign + cm.ObjectMeta.ResourceVersion
	}

	// Add SelfSigned certificate for a git repository to the bundle
	if ctx.CheCluster.Spec.DevEnvironments.TrustedCerts != nil && ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName != "" {
		gitTrustedCertsConfig := &corev1.ConfigMap{}
		exists, err := deploy.GetNamespacedObject(ctx, ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName, gitTrustedCertsConfig)
		if err != nil {
			return false, err
		} else if exists && gitTrustedCertsConfig.Data["ca.crt"] != "" {
			if revisions != "" {
				revisions += labelCommaSign
			}
			revisions += gitTrustedCertsConfig.Name + labelEqualSign + gitTrustedCertsConfig.ResourceVersion

			data[gitTrustedCertsConfig.Name+".ca.crt"] = gitTrustedCertsConfig.Data["ca.crt"]
		}
	}

	mergedCAConfigMapSpec := deploy.GetConfigMapSpec(ctx, CheAllCACertsConfigMapName, data, defaults.GetCheFlavor())
	mergedCAConfigMapSpec.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
	mergedCAConfigMapSpec.ObjectMeta.Annotations[CheMergedCAConfigMapRevisionsAnnotationKey] = revisions
	return deploy.SyncConfigMapSpecToCluster(ctx, mergedCAConfigMapSpec)
}
