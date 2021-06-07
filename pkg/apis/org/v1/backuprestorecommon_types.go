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
)

// +k8s:openapi-gen=true
// List of supported backup servers
type BackupServers struct {
	// Rest server within the cluster.
	// The server and configuration are created by operator when AutoconfigureRestBackupServer is true.
	Internal RestServerConfig `json:"internal,omitempty"`
	// Sftp backup server configuration.
	// Mandatory fields are: RepoPassword, Hostname, Repo, Username, SshKeySecretRef.
	Sftp SftpServerConfing `json:"sftp,omitempty"`
	// Rest backup server configuration.
	// Mandatory fields are: RepoPassword, Hostname.
	Rest RestServerConfig `json:"rest,omitempty"`
	// Amazon S3 or alternatives.
	// Mandatory fields are: RepoPassword, Repo, AWS key+id or secret with it.
	AwsS3 AwsS3ServerConfig `json:"awss3,omitempty"`
}

// +k8s:openapi-gen=true
// SFTP backup server configuration
// Example: user@host://srv/repo
// Mandatory fields are: ResticRepoPasswordSecretRef, Hostname, Repo, Username, SshKeySecretRef
type SftpServerConfing struct {
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	ResticRepoPasswordSecretRef string `json:"resticRepoPasswordSecretRef,omitempty"`
	// Backup server host
	Hostname string `json:"hostname,omitempty"`
	// Backup server port
	Port int `json:"port,omitempty"`
	// Restic repository path, relative or absolute, e.g. /srv/repo
	Repo string `json:"repo,omitempty"`
	// User login on the remote server
	Username string `json:"username,omitempty"`
	// Private ssh key under 'ssh-privatekey' field for passwordless login
	SshKeySecretRef string `json:"sshKeySecretRef,omitempty"`
}

// +k8s:openapi-gen=true
// REST backup server configuration
// Example: https://user:password@host:5000/repo/
// Mandatory fields are: ResticRepoPasswordSecretRef, Hostname
type RestServerConfig struct {
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	ResticRepoPasswordSecretRef string `json:"resticRepoPasswordSecretRef,omitempty"`
	// Protocol to use when connection to the server
	// Defaults to https.
	Protocol string `json:"protocol,omitempty"`
	// Backup server host
	Hostname string `json:"hostname,omitempty"`
	// Backup server port
	Port int `json:"port,omitempty"`
	// Restic repository path
	Repo string `json:"repo,omitempty"`
	// Secret that contains username and password fields to login into restic server.
	// Note, each repository is encrypted with own password. See ResticRepoPasswordSecretRef field.
	CredentialsSecretRef string `json:"credentialsSecretRef,omitempty"`
}

// +k8s:openapi-gen=true
// Mandatory fields are: ResticRepoPasswordSecretRef, Repo, AwsAccessKeySecretRef
type AwsS3ServerConfig struct {
	// Holds reference to a secret with restic repository password under 'repo-password' field to encrypt / decrypt its content.
	ResticRepoPasswordSecretRef string `json:"resticRepoPasswordSecretRef,omitempty"`
	// Protocol to use when connection to the server.
	// Might be customized in case of alternative server.
	Protocol string `json:"protocol,omitempty"`
	// Server hostname, defaults to 's3.amazonaws.com'.
	// Might be customized in case of alternative server.
	Hostname string `json:"hostname,omitempty"`
	// Backup server port.
	// Usually default value is used.
	// Might be customized in case of alternative server.
	Port int `json:"port,omitempty"`
	// Bucket name and repository, e.g. bucket/repo
	Repo string `json:"repo,omitempty"`
	// Reference to secret that contains awsAccessKeyId and awsSecretAccessKey keys.
	AwsAccessKeySecretRef string `json:"awsAccessKeySecretRef,omitempty"`
}
