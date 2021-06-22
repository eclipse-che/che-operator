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

// +k8s:openapi-gen=true
// CheClusterBackupSpec defines the desired state of CheClusterBackup
type CheClusterBackupSpec struct {
	// Automatically setup pod with REST backup server and use the server in this configuration.
	// Note, this flag takes precedence and will overwrite existing backup server configuration.
	// +optional
	UseInternalBackupServer bool `json:"useInternalBackupServer,omitempty"`
	// Name of custom resource with a backup server configuration to use for this backup.
	// Note, UseInternalBackupServer field can configure internal backup server automatically.
	// +optional
	BackupServerConfigRef string `json:"backupServerConfigRef,omitempty"`
}

// +k8s:openapi-gen=true
// CheClusterBackupStatus defines the observed state of CheClusterBackup
type CheClusterBackupStatus struct {
	// Message explaining the state of the backup or an error message
	// +optional
	Message string `json:"message,omitempty"`
	// Backup progress state: InProgress, Failed, Succeeded
	// +optional
	State string `json:"state,omitempty"`
	// Describes backup progress
	// +optional
	Phase string `json:"stage,omitempty"`
	// Last backup snapshot ID
	// +optional
	SnapshotId string `json:"snapshotId,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheClusterBackup is the Schema for the checlusterbackups API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=checlusterbackups,scope=Namespaced
type CheClusterBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheClusterBackupSpec   `json:"spec,omitempty"`
	Status CheClusterBackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheClusterBackupList contains a list of CheClusterBackup
type CheClusterBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheClusterBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheClusterBackup{}, &CheClusterBackupList{})
}
