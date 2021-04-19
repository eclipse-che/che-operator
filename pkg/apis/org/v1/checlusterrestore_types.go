package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "operator-sdk generate k8s" and "operator-sdk generate crds" to regenerate code after modifying this file

// CheClusterRestoreSpec defines the desired state of CheClusterRestore
type CheClusterRestoreSpec struct {
	// If true deletes the CR after successful restore.
	DeleteConfigurationAfterRestore bool `json:"deleteConfigurationAfterRestore,omitempty"`
	// Snapshit it to restore from.
	// If omitted, latest snapshot will be used.
	SnapshotId string `json:"snapshotId,omitempty"`
	// Set to true to start backup process.
	TriggerNow bool `json:"triggerNow,omitempty"`
	// If more than one backup server configured, should specify which one to use.
	// Allowed values are fields names form BackupServers struct.
	ServerType string `json:"serverType,omitempty"`
	// List of backup servers.
	// Usually only one is used.
	// In case of several available, serverType should contain server to use.
	Servers BackupServers `json:"servers"`
}

// CheClusterRestoreStatus defines the observed state of CheClusterRestore
type CheClusterRestoreStatus struct {
	// Backup result or error message
	Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheClusterRestore is the Schema for the checlusterrestores API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=checlusterrestores,scope=Namespaced
type CheClusterRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheClusterRestoreSpec   `json:"spec,omitempty"`
	Status CheClusterRestoreStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheClusterRestoreList contains a list of CheClusterRestore
type CheClusterRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheClusterRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheClusterRestore{}, &CheClusterRestoreList{})
}
