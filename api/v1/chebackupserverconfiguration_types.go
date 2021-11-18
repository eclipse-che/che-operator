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

const (
	USERNAME_SECRET_KEY              = "username"
	PASSWORD_SECRET_KEY              = "password"
	RESTIC_REPO_PASSWORD_SECRET_KEY  = "repo-password"
	SSH_PRIVATE_KEY_SECRET_KEY       = "ssh-privatekey"
	AWS_ACCESS_KEY_ID_SECRET_KEY     = "awsAccessKeyId"
	AWS_SECRET_ACCESS_KEY_SECRET_KEY = "awsSecretAccessKey"
)

// CheBackupServerConfigurationSpec defines the desired state of CheBackupServerConfiguration
// Only one type of backup server is allowed to be configured per CR.
type CheBackupServerConfigurationSpec struct {
	// Rest backup server configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Rest server"
	Rest *RestServerConfig `json:"rest,omitempty"`
	// Amazon S3 or compatible alternatives.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="AwsS3 server"
	AwsS3 *AwsS3ServerConfig `json:"awss3,omitempty"`
	// Sftp backup server configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Sftp server"
	Sftp *SftpServerConfing `json:"sftp,omitempty"`
}

// +k8s:openapi-gen=true
// REST backup server configuration
// Examples: host:5000/repo/ https://user:password@host:5000/repo/
type RestServerConfig struct {
	// Protocol to use when connection to the server
	// Defaults to https.
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Backup server host
	Hostname string `json:"hostname"`
	// Backup server port
	// +optional
	Port int `json:"port,omitempty"`
	// Restic repository path
	// +optional
	RepositoryPath string `json:"repositoryPath,omitempty"`
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	RepositoryPasswordSecretRef string `json:"repositoryPasswordSecretRef"`
	// Secret that contains username and password fields to login into restic server.
	// Note, each repository is encrypted with own password. See ResticRepoPasswordSecretRef field.
	// +optional
	CredentialsSecretRef string `json:"credentialsSecretRef,omitempty"`
}

// +k8s:openapi-gen=true
// Examples: http://server:port/bucket/repo
type AwsS3ServerConfig struct {
	// Protocol to use when connection to the server.
	// Might be customized in case of alternative server.
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Server hostname, defaults to 's3.amazonaws.com'.
	// Might be customized in case of alternative server.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Backup server port.
	// Usually default value is used.
	// Might be customized in case of alternative server.
	// +optional
	Port int `json:"port,omitempty"`
	// Bucket name and repository, e.g. bucket/repo
	RepositoryPath string `json:"repositoryPath"`
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	RepositoryPasswordSecretRef string `json:"repositoryPasswordSecretRef"`
	// Reference to secret that contains awsAccessKeyId and awsSecretAccessKey keys.
	AwsAccessKeySecretRef string `json:"awsAccessKeySecretRef"`
}

// +k8s:openapi-gen=true
// SFTP backup server configuration
// Examples: user@host:/srv/repo user@host:1234//srv/repo
type SftpServerConfing struct {
	// User login on the remote server
	Username string `json:"username"`
	// Backup server host
	Hostname string `json:"hostname"`
	// Backup server port
	// +optional
	Port int `json:"port,omitempty"`
	// Restic repository path, relative or absolute, e.g. /srv/repo
	RepositoryPath string `json:"repositoryPath"`
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	RepositoryPasswordSecretRef string `json:"repositoryPasswordSecretRef"`
	// Private ssh key under 'ssh-privatekey' field for passwordless login
	SshKeySecretRef string `json:"sshKeySecretRef"`
}

// CheBackupServerConfigurationStatus defines the observed state of CheBackupServerConfiguration
type CheBackupServerConfigurationStatus struct {
}

// The `CheBackupServerConfiguration` custom resource allows defining and managing Eclipse Che Backup Server Configurations
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +operator-sdk:csv:customresourcedefinitions:displayName="Eclipse Che Backup Server"
// +operator-sdk:csv:customresourcedefinitions:order=1
// +operator-sdk:csv:customresourcedefinitions:resources={}
type CheBackupServerConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheBackupServerConfigurationSpec   `json:"spec,omitempty"`
	Status CheBackupServerConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CheBackupServerConfigurationList contains a list of CheBackupServerConfiguration
type CheBackupServerConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheBackupServerConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheBackupServerConfiguration{}, &CheBackupServerConfigurationList{})
}
