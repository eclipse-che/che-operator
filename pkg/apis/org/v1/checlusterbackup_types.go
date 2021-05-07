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

// Important: Run "operator-sdk generate k8s" and "operator-sdk generate crds" to regenerate code after modifying this file

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:openapi-gen=true
// CheClusterBackupSpec defines the desired state of CheClusterBackup
type CheClusterBackupSpec struct {
	// Automatically setup container with REST backup server and use the server in this configuration.
	// Note, this will overwrite existing configuration.
	AutoconfigureRestBackupServer bool `json:"autoconfigureRestBackupServer,omitempty"`
	// Set to true to start backup process.
	TriggerNow bool `json:"triggerNow"`
	// If more than one backup server configured, should specify which one to use.
	// Allowed values are fields names form BackupServers struct.
	ServerType string `json:"serverType,omitempty"`
	// List of backup servers.
	// Usually only one is used.
	// In case of several available, ServerType should contain server to use.
	Servers BackupServers `json:"servers,omitempty"`
}

// +k8s:openapi-gen=true
// CheClusterBackupStatus defines the observed state of CheClusterBackup
type CheClusterBackupStatus struct {
	// Backup result or error message
	Message string `json:"message,omitempty"`
	// Shows when backup was done last time
	LastBackupTime string `json:"lastBackupTime,omitempty"`
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
