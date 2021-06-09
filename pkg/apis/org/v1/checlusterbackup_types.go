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

// Important: when any changes are made, CRD must be regenerated.
// Please use olm/update-resources.sh script for that.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:openapi-gen=true
// CheClusterBackupSpec defines the desired state of CheClusterBackup
type CheClusterBackupSpec struct {
	// Automatically setup pod with REST backup server and use the server in this configuration.
	// Note, this will overwrite existing configuration.
	// +optional
	UseInternalBackupServer bool `json:"useInternalBackupServer,omitempty"`
	// Set to true to start backup process.
	// +optional
	TriggerNow bool `json:"triggerNow"`
	// List of backup servers.
	// Only one backup server is allowed to configure at a time.
	// Note, UseInternalBackupServer field can configure internal backup server.
	// +optional
	BackupServerConfig BackupServersConfigs `json:"servers,omitempty"`
}

// +k8s:openapi-gen=true
// CheClusterBackupStatus defines the observed state of CheClusterBackup
type CheClusterBackupStatus struct {
	// Message explaining the state of the backup or an error message
	// +optional
	Message string `json:"message,omitempty"`
	// Backup progress state: InProgress, Failed, Successed
	State string `json:"state,omitempty"`
	// Last backup snapshot ID
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
