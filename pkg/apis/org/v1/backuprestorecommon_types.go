//
// Copyright (c) 2012-2019 Red Hat, Inc.
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

// +k8s:openapi-gen=true
// List of supported backup servers
type BackupServers struct {
	// Rest server within the cluster.
	// The server and configuration are created by operator when AutoconfigureRestBackupServer is true
	Internal RestServerConfing `json:"internal,omitempty"`
	// Sftp backup server configuration
	Sftp SftpServerConfing `json:"sftp,omitempty"`
	// Rest backup server configuration
	Rest RestServerConfing `json:"rest,omitempty"`
	// Amazon S3 or alternatives
	AwsS3 AwsS3ServerConfig `json:"awss3,omitempty"`
}

// Holds restic repository password to decrypt its content
type RepoPassword struct {
	// Password for restic repository
	RepoPassword string `json:"repoPassword,omitempty"`
	// Secret with 'repo-password' filed
	RepoPasswordSecretRef string `json:"repoPasswordSecretRef,omitempty"`
}

// +k8s:openapi-gen=true
// SFTP backup server configuration
// Example: user@host://srv/repo
type SftpServerConfing struct {
	RepoPassword `json:"repoPassword"`
	// Backup server host
	Hostname string `json:"hostname"`
	// Backup server port
	Port string `json:"port,omitempty"`
	// Restic repository path, relative or absolute, e.g. /srv/repo
	Repo string `json:"repo"`
	// User login on the remote server
	Username string `json:"username"`
	// Private ssh key under 'ssh-key' field for passwordless login
	SshKeySecretRef string `json:"sshKeySecretRef"`
}

// +k8s:openapi-gen=true
// REST backup server configuration
// Example: https://user:password@host:5000/repo/
type RestServerConfing struct {
	RepoPassword `json:"repoPassword"`
	// Protocol to use when connection to the server
	// Defaults to https.
	Protocol string `json:"protocol,omitempty"`
	// Backup server host
	Hostname string `json:"hostname"`
	// Backup server port
	Port string `json:"port,omitempty"`
	// Restic repository path
	Repo string `json:"repo,omitempty"`
	// User login on the remote server
	Username string `json:"username,omitempty"`
	// Password to authenticate the user
	Password string `json:"password,omitempty"`
	// Secret that contains username and password fields
	CredentialsSecretRef string `json:"credentialsSecretRef,omitempty"`
}

// +k8s:openapi-gen=true
type AwsS3ServerConfig struct {
	RepoPassword `json:"repoPassword"`
	// Protocol to use when connection to the server.
	// Might be customized in case of alternative server.
	Protocol string `json:"protocol,omitempty"`
	// Server hostname, defaults to 's3.amazonaws.com'.
	// Might be customized in case of alternative server.
	Hostname string `json:"hostname,omitempty"`
	// Backup server port.
	// Usually default value is used.
	// Might be customized in case of alternative server.
	Port string `json:"port,omitempty"`
	// Bucket name and repository, e.g. bucket/repo
	Repo string `json:"repo"`
	// Content of AWS_ACCESS_KEY_ID environment variable
	AwsAccessKeyId string `json:"awsAccessKeyId,omitempty"`
	// Content of AWS_SECRET_ACCESS_KEY environment variable
	AwsSecretAccessKey string `json:"awsSecretAccessKey,omitempty"`
	// Reference to secret that contains awsAccessKeyId and awsSecretAccessKey fields
	AwsAccessKeySecretRef string `json:"awsAccessKeySecretRef,omitempty"`
}
