//
// Copyright (c) 2020 Red Hat, Inc.
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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var configMapDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels)
	}),
}

func SyncConfigMapDataToCluster(
	deployContext *DeployContext,
	name string,
	data map[string]string,
	component string) (bool, error) {

	configMapSpec := GetConfigMapSpec(deployContext, name, data, component)
	return Sync(deployContext, configMapSpec, configMapDiffOpts)
}

func SyncConfigMapSpecToCluster(
	deployContext *DeployContext,
	configMapSpec *corev1.ConfigMap) (bool, error) {

	return Sync(deployContext, configMapSpec, configMapDiffOpts)
}

func GetConfigMapSpec(
	deployContext *DeployContext,
	name string,
	data map[string]string,
	component string) *corev1.ConfigMap {

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   deployContext.CheCluster.Namespace,
			Labels:      GetLabels(deployContext.CheCluster, component),
			Annotations: map[string]string{},
		},
		Data: data,
	}

	return configMap
}
