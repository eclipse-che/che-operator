//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

// CheClusterBackupSpec defines the desired state of CheClusterBackup
type CheClusterBackupSpec struct {
	// Automatically setup pod with REST backup server and use the server in this configuration.
	// Note, this flag takes precedence and will overwrite existing backup server configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Use internal backup server"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	UseInternalBackupServer bool `json:"useInternalBackupServer,omitempty"`
	// Name of custom resource with a backup server configuration to use for this backup.
	// Note, UseInternalBackupServer field can configure internal backup server automatically.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Backup server configuration"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:text"
	BackupServerConfigRef string `json:"backupServerConfigRef,omitempty"`
}

// CheClusterBackupStatus defines the observed state of CheClusterBackup
type CheClusterBackupStatus struct {
	// Message explaining the state of the backup or an error message
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:displayName="Message"
	// +operator-sdk:csv:customresourcedefinitions:xDescriptors="urn:alm:descriptor:io.kubernetes.phase:reason"
	Message string `json:"message,omitempty"`
	// Backup progress state: InProgress, Failed, Succeeded
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:displayName="Backup state"
	// +operator-sdk:csv:customresourcedefinitions:xDescriptors="urn:alm:descriptor:io.kubernetes.phase"
	State string `json:"state,omitempty"`
	// Describes backup progress
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:displayName="Backup progress"
	// +operator-sdk:csv:customresourcedefinitions:xDescriptors="urn:alm:descriptor:text"
	Phase string `json:"stage,omitempty"`
	// Last backup snapshot ID
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:displayName="Backup snapshot Id"
	// +operator-sdk:csv:customresourcedefinitions:xDescriptors="urn:alm:descriptor:text"
	SnapshotId string `json:"snapshotId,omitempty"`
	// Version that was backed up
	// +optional
	CheVersion string `json:"cheVersion,omitempty"`
}

// The `CheClusterBackup` custom resource allows defining and managing Eclipse Che backup
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +operator-sdk:csv:customresourcedefinitions:displayName="Eclipse Che instance Backup Specification"
// +operator-sdk:csv:customresourcedefinitions:order=2
// +operator-sdk:csv:customresourcedefinitions:resources={{Service,v1,backup-rest-server-service},{Deployment,apps/v1,backup-rest-server-deployment}}
type CheClusterBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheClusterBackupSpec   `json:"spec,omitempty"`
	Status CheClusterBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CheClusterBackupList contains a list of CheClusterBackup
type CheClusterBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheClusterBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheClusterBackup{}, &CheClusterBackupList{})
}
