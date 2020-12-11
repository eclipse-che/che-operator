package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KubernetesImagePullerSpec defines the desired state of KubernetesImagePuller
type KubernetesImagePullerSpec struct {
	ConfigMapName        string `json:"configMapName,omitempty"`
	DaemonsetName        string `json:"daemonsetName,omitempty"`
	DeploymentName       string `json:"deploymentName,omitempty"`
	Images               string `json:"images,omitempty"`
	CachingIntervalHours string `json:"cachingIntervalHours,omitempty"`
	CachingMemoryRequest string `json:"cachingMemoryRequest,omitempty"`
	CachingMemoryLimit   string `json:"cachingMemoryLimit,omitempty"`
	CachingCpuRequest    string `json:"cachingCPURequest,omitempty"`
	CachingCpuLimit      string `json:"cachingCPULimit,omitempty"`
	NodeSelector         string `json:"nodeSelector,omitempty"`
}

// KubernetesImagePullerStatus defines the observed state of KubernetesImagePuller
type KubernetesImagePullerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesImagePuller is the Schema for the kubernetesimagepullers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kubernetesimagepullers,scope=Namespaced
type KubernetesImagePuller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesImagePullerSpec   `json:"spec,omitempty"`
	Status KubernetesImagePullerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
