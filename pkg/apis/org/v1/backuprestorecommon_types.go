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

const (
	USERNAME_SECRET_KEY              = "username"
	PASSWORD_SECRET_KEY              = "password"
	RESTIC_REPO_PASSWORD_SECRET_KEY  = "repo-password"
	SSH_PRIVATE_KEY_SECRET_KEY       = "ssh-privatekey"
	AWS_ACCESS_KEY_ID_SECRET_KEY     = "awsAccessKeyId"
	AWS_SECRET_ACCESS_KEY_SECRET_KEY = "awsSecretAccessKey"

	STATE_IN_PROGRESS = "InProgress"
	STATE_SUCCEEDED   = "Succeeded"
	STATE_FAILED      = "Failed"
)

// +k8s:openapi-gen=true
// List of supported backup servers
type BackupServersConfigs struct {
	// Sftp backup server configuration.
	// Mandatory fields are: Username, Hostname, RepositoryPath, RepositoryPasswordSecretRef, SshKeySecretRef.
	// +optional
	Sftp *SftpServerConfing `json:"sftp,omitempty"`
	// Rest backup server configuration.
	// Mandatory fields are: Hostname, RepositoryPasswordSecretRef.
	// +optional
	Rest *RestServerConfig `json:"rest,omitempty"`
	// Amazon S3 or compatible alternatives.
	// Mandatory fields are: RepositoryPasswordSecretRef, RepositoryPath, CredentialsSecretRef.
	// +optional
	AwsS3 *AwsS3ServerConfig `json:"awss3,omitempty"`
}

// +k8s:openapi-gen=true
// SFTP backup server configuration
// Example: user@host://srv/repo
// Mandatory fields are: Username, Hostname, RepositoryPath, RepositoryPasswordSecretRef, SshKeySecretRef
type SftpServerConfing struct {
	// User login on the remote server
	// +optional
	Username string `json:"username,omitempty"`
	// Backup server host
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Backup server port
	// +optional
	Port int `json:"port,omitempty"`
	// Restic repository path, relative or absolute, e.g. /srv/repo
	// +optional
	RepositoryPath string `json:"repositoryPath,omitempty"`
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	// +optional
	RepositoryPasswordSecretRef string `json:"repositoryPasswordSecretRef,omitempty"`
	// Private ssh key under 'ssh-privatekey' field for passwordless login
	// +optional
	SshKeySecretRef string `json:"sshKeySecretRef,omitempty"`
}

// +k8s:openapi-gen=true
// REST backup server configuration
// Example: https://user:password@host:5000/repo/
// Mandatory fields are: Hostname, RepositoryPasswordSecretRef.
type RestServerConfig struct {
	// Protocol to use when connection to the server
	// Defaults to https.
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Backup server host
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Backup server port
	// +optional
	Port int `json:"port,omitempty"`
	// Restic repository path
	// +optional
	RepositoryPath string `json:"repositoryPath,omitempty"`
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	// +optional
	RepositoryPasswordSecretRef string `json:"repositoryPasswordSecretRef,omitempty"`
	// Secret that contains username and password fields to login into restic server.
	// Note, each repository is encrypted with own password. See ResticRepoPasswordSecretRef field.
	// +optional
	CredentialsSecretRef string `json:"credentialsSecretRef,omitempty"`
}

// +k8s:openapi-gen=true
// Mandatory fields are: RepositoryPasswordSecretRef, RepositoryPath, CredentialsSecretRef.
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
	// +optional
	RepositoryPath string `json:"repositoryPath,omitempty"`
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	// +optional
	RepositoryPasswordSecretRef string `json:"repositoryPasswordSecretRef,omitempty"`
	// Reference to secret that contains awsAccessKeyId and awsSecretAccessKey keys.
	// +optional
	AwsAccessKeySecretRef string `json:"awsAccessKeySecretRef,omitempty"`
}
