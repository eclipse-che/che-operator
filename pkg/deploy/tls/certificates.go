//
// Copyright (c) 2012-2021 Red Hat, Inc.
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

	// Local constants
	// labelEqualSign consyant is used as a replacement for '=' symbol in labels because '=' is not allowed there
	labelEqualSign = "-"
	// labelCommaSign consyant is used as a replacement for ',' symbol in labels because ',' is not allowed there
	labelCommaSign = "."
)

type CertificatesReconciler struct {
	deploy.Reconcilable
}

func NewCertificatesReconciler() *CertificatesReconciler {
	return &CertificatesReconciler{}
}

func (c *CertificatesReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if ctx.Proxy.TrustedCAMapName != "" {
		if ctx.CheCluster.Spec.Server.ServerTrustStoreConfigMapName == "" {
			ctx.CheCluster.Spec.Server.ServerTrustStoreConfigMapName = deploy.DefaultServerTrustStoreConfigMapName()
			if err := deploy.UpdateCheCRSpec(ctx, "ServerTrustStoreConfigMapName", deploy.DefaultServerTrustStoreConfigMapName()); err != nil {
				return reconcile.Result{}, false, err
			}

			actual := &corev1.ConfigMap{}
			err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: deploy.DefaultServerTrustStoreConfigMapName()}, actual)
			if err == nil {
				if actual.ObjectMeta.Labels[deploy.KubernetesPartOfLabelKey] != deploy.CheEclipseOrg {
					if actual.ObjectMeta.Labels == nil {
						actual.ObjectMeta.Labels = make(map[string]string)
					}
					actual.ObjectMeta.Labels[deploy.KubernetesPartOfLabelKey] = deploy.CheEclipseOrg
					err := ctx.ClusterAPI.NonCachingClient.Update(context.TODO(), actual)
					return reconcile.Result{}, false, err
				}
			}
		}

		result, done, err := c.syncTrustStoreConfigMapToCluster(ctx)
		if !done {
			return result, done, err
		}
	}

	return c.syncAdditionalCACertsConfigMapToCluster(ctx)
}

func (c *CertificatesReconciler) Finalize(ctx *deploy.DeployContext) error {
	return nil
}

func (c *CertificatesReconciler) syncTrustStoreConfigMapToCluster(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	name := ctx.CheCluster.Spec.Server.ServerTrustStoreConfigMapName
	configMapSpec := deploy.GetConfigMapSpec(ctx, name, map[string]string{}, deploy.DefaultCheFlavor(ctx.CheCluster))

	// OpenShift will automatically injects all certs into the configmap
	configMapSpec.ObjectMeta.Labels[injector] = "true"
	configMapSpec.ObjectMeta.Labels[deploy.KubernetesPartOfLabelKey] = deploy.CheEclipseOrg
	configMapSpec.ObjectMeta.Labels[deploy.KubernetesComponentLabelKey] = CheCACertsConfigMapLabelValue

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(ctx, name, actual)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	if !exists {
		// We have to create an empty config map with the specific labels
		done, err := deploy.Create(ctx, configMapSpec)
		return reconcile.Result{}, done, err
	}

	if actual.ObjectMeta.Labels[injector] != "true" ||
		actual.ObjectMeta.Labels[deploy.KubernetesPartOfLabelKey] != deploy.CheEclipseOrg ||
		actual.ObjectMeta.Labels[deploy.KubernetesComponentLabelKey] != CheCACertsConfigMapLabelValue {

		actual.ObjectMeta.Labels[injector] = "true"
		actual.ObjectMeta.Labels[deploy.KubernetesPartOfLabelKey] = deploy.CheEclipseOrg
		actual.ObjectMeta.Labels[deploy.KubernetesComponentLabelKey] = CheCACertsConfigMapLabelValue

		logrus.Infof("Updating existed object: %s, name: %s", configMapSpec.Kind, configMapSpec.Name)
		if err := ctx.ClusterAPI.Client.Update(context.TODO(), actual); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (c *CertificatesReconciler) syncAdditionalCACertsConfigMapToCluster(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	// Get all source config maps, if any
	caConfigMaps, err := GetCACertsConfigMaps(ctx.ClusterAPI.Client, ctx.CheCluster.GetNamespace())
	if err != nil {
		return reconcile.Result{}, false, err
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
			return reconcile.Result{}, true, nil
		}
	} else {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, false, err
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

	mergedCAConfigMapSpec := deploy.GetConfigMapSpec(ctx, CheAllCACertsConfigMapName, data, deploy.DefaultCheFlavor(ctx.CheCluster))
	mergedCAConfigMapSpec.ObjectMeta.Labels[deploy.KubernetesPartOfLabelKey] = deploy.CheEclipseOrg
	mergedCAConfigMapSpec.ObjectMeta.Annotations[CheMergedCAConfigMapRevisionsAnnotationKey] = revisions
	done, err := deploy.SyncConfigMapSpecToCluster(ctx, mergedCAConfigMapSpec)
	return reconcile.Result{}, done, err
}
