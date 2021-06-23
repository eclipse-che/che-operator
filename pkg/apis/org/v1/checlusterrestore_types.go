//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheClusterRestoreSpec defines the desired state of CheClusterRestore
type CheClusterRestoreSpec struct {
	// Snapshot ID to restore from.
	// If omitted, latest snapshot will be used.
	// +optional
	SnapshotId string `json:"snapshotId,omitempty"`
	// Name of custom resource with a backup server configuration to use for this restore.
	// Can be omitted if only one server configuration object exists within the namespace.
	// +optional
	BackupServerConfigRef string `json:"backupServerConfigRef,omitempty"`
}

// CheClusterRestoreStatus defines the observed state of CheClusterRestore
type CheClusterRestoreStatus struct {
	// Restore result or error message
	// +optional
	Message string `json:"message,omitempty"`
	// Describes phase of restore progress
	// +optional
	Phase string `json:"stage,omitempty"`
	// Restore progress state: InProgress, Failed, Succeeded
	// +optional
	State string `json:"state,omitempty"`
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
