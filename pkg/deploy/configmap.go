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

package deploy

import (
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ConfigMapDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels)
	}),
}

// InitConfigMap returns a new ConfigMap Kubernetes object.
func InitConfigMap(
	ctx *chetypes.DeployContext,
	name string,
	data map[string]string,
	component string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ctx.CheCluster.Namespace,
			Labels:      GetLabels(component),
			Annotations: map[string]string{},
		},
		Data: data,
	}
}

func SyncConfigMapDataToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	data map[string]string,
	component string) (bool, error) {

	configMapSpec := InitConfigMap(deployContext, name, data, component)
	return Sync(deployContext, configMapSpec, ConfigMapDiffOpts)
}

func SyncConfigMapSpecToCluster(
	deployContext *chetypes.DeployContext,
	configMapSpec *corev1.ConfigMap) (bool, error) {

	return Sync(deployContext, configMapSpec, ConfigMapDiffOpts)
}

// SyncConfigMap synchronizes the ConfigMap with the cluster.
func SyncConfigMap(
	ctx *chetypes.DeployContext,
	cm *corev1.ConfigMap,
	ensuredLabels []string,
	ensuredAnnotations []string) (bool, error) {

	diffs := cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		GetLabelsAndAnnotationsComparator(ensuredLabels, ensuredAnnotations),
	}

	return Sync(ctx, cm, diffs)
}

// SyncConfigMapIgnoreData synchronizes the ConfigMap with the cluster ignoring the data field.
func SyncConfigMapIgnoreData(
	ctx *chetypes.DeployContext,
	cm *corev1.ConfigMap,
	ensuredLabels []string,
	ensuredAnnotations []string) (bool, error) {

	diffs := cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "Data"),
		GetLabelsAndAnnotationsComparator(ensuredLabels, ensuredAnnotations),
	}

	return Sync(ctx, cm, diffs)
}
