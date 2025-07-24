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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KubernetesImagePullerSpec defines the desired state of KubernetesImagePuller
type KubernetesImagePullerSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ConfigMap name"
	ConfigMapName string `json:"configMapName,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DaemonSet name"
	DaemonsetName string `json:"daemonsetName,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Deployment name"
	DeploymentName string `json:"deploymentName,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Images to pull"
	Images string `json:"images,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Caching internal hours"
	CachingIntervalHours string `json:"cachingIntervalHours,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Caching memory request"
	CachingMemoryRequest string `json:"cachingMemoryRequest,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cache memory limit"
	CachingMemoryLimit string `json:"cachingMemoryLimit,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Caching CPU request"
	CachingCpuRequest string `json:"cachingCPURequest,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Caching CPU limit"
	CachingCpuLimit string `json:"cachingCPULimit,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodeSelector"
	NodeSelector string `json:"nodeSelector,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePull secrets"
	ImagePullSecrets string `json:"imagePullSecrets,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Affinity"
	Affinity string `json:"affinity,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ImagePull name"
	ImagePullerImage string `json:"imagePullerImage,omitempty"`
}

// KubernetesImagePullerStatus defines the observed state of KubernetesImagePuller
type KubernetesImagePullerStatus struct {
	// KubernetesImagePuller image in use.
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Image"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	ImagePullerImage string `json:"imagePullerImage,omitempty"`
}

// KubernetesImagePuller is the Schema for the kubernetesimagepullers API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kubernetesimagepullers,scope=Namespaced
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1},{Deployment,apps/v1},{DaemonSet,apps/v1}}
type KubernetesImagePuller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesImagePullerSpec   `json:"spec,omitempty"`
	Status KubernetesImagePullerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubernetesImagePullerList contains a list of KubernetesImagePuller
type KubernetesImagePullerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesImagePuller `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesImagePuller{}, &KubernetesImagePullerList{})
}

type KubernetesImagePullerConfig struct {
	configMap *corev1.ConfigMap
}

func (config *KubernetesImagePullerConfig) WithDaemonsetName(name string) *KubernetesImagePullerConfig {
	config.configMap.Data["DAEMONSET_NAME"] = name
	return &KubernetesImagePullerConfig{
		configMap: config.configMap,
	}
}
