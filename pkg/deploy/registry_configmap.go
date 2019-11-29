//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"encoding/json"
	"fmt"
	"github.com/eclipse/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

type DevFileRegistryConfigMap struct {
	CheDevfileImagesRegistryURL          string `json:"CHE_DEVFILE_IMAGES_REGISTRY_URL"`
	CheDevfileImagesRegistryOrganization string `json:"CHE_DEVFILE_IMAGES_REGISTRY_ORGANIZATION"`
	CheDevfileRegistryURL                string `json:"CHE_DEVFILE_REGISTRY_URL"`
}

type PluginRegistryConfigMap struct {
	CheSidecarContainersRegistryURL          string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_URL"`
	CheSidecarContainersRegistryOrganization string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_ORGANIZATION"`
}

func CreateDevfileRegistryConfigMap(cr *orgv1.CheCluster, endpoint string) *corev1.ConfigMap {
	labels := GetLabels(cr, util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor))
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devfile-registry",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: GetDevfileRegistryConfigMapData(cr, endpoint),
	}
}

func CreatePluginRegistryConfigMap(cr *orgv1.CheCluster) *corev1.ConfigMap {
	labels := GetLabels(cr, util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor))
	fmt.Println("Cr namespace " + cr.Namespace)
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plugin-registry",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: GetPluginRegistryConfigMapData(cr),
	}
}

func GetDevfileRegistryConfigMapData(cr *orgv1.CheCluster, endpoint string) map[string]string {
	devfileRegistryEnv := make(map[string]string)
	data := &DevFileRegistryConfigMap{
		CheDevfileImagesRegistryURL:          cr.Spec.Server.AirGapContainerRegistryHostname,
		CheDevfileImagesRegistryOrganization: cr.Spec.Server.AirGapContainerRegistryOrganization,
		CheDevfileRegistryURL:                endpoint,
	}

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}

	err = json.Unmarshal(out, &devfileRegistryEnv)
	if err != nil {
		fmt.Println(err)
	}
	return devfileRegistryEnv
}

func GetPluginRegistryConfigMapData(cr *orgv1.CheCluster) map[string]string {
	pluginRegistryEnv := make(map[string]string)
	data := &PluginRegistryConfigMap{
		CheSidecarContainersRegistryURL:          cr.Spec.Server.AirGapContainerRegistryHostname,
		CheSidecarContainersRegistryOrganization: cr.Spec.Server.AirGapContainerRegistryOrganization,
	}

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}

	err = json.Unmarshal(out, &pluginRegistryEnv)
	if err != nil {
		fmt.Println(err)
	}

	return pluginRegistryEnv
}
