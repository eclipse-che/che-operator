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
package backup_servers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SftpServer implements BackupServer
type SftpServer struct {
	config       orgv1.SftpServerConfing
	repoPassword string
	url          string
}

func (s *SftpServer) PrepareConfiguration(client client.Client, namespace string) (bool, error) {
	repoPassword, done, err := getResticRepoPassword(client, namespace, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}
	s.repoPassword = repoPassword

	user := s.config.Username
	if user == "" {
		return true, fmt.Errorf("SFTP server username must be configured")
	}
	host := s.config.Hostname
	if host == "" {
		return true, fmt.Errorf("SFTP server hostname must be configured")
	}
	port := getPortString(s.config.Port)
	path := s.config.Repo
	if path == "" {
		return true, fmt.Errorf("repository (path on server side) must be configured")
	}

	sshKey := ""
	if s.config.SshKeySecretRef == "" {
		return true, fmt.Errorf("secret with SSH key is not specified. It is mandatory to connect to SFTP backup server")
	}
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: s.config.SshKeySecretRef}
	if err := client.Get(context.TODO(), namespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			return true, fmt.Errorf("secret '%s' with SSH key not found", s.config.SshKeySecretRef)
		}
		return false, err
	}
	if value, exists := secret.Data["ssh-key"]; exists {
		sshKey = string(value)
	} else {
		if len(secret.Data) == 1 {
			// Use the only one field in the secret as ssh key
			for _, password := range secret.Data {
				sshKey = string(password)
				break
			}
		} else {
			return true, fmt.Errorf("'%s' secret should have 'ssh-key' field", s.config.SshKeySecretRef)
		}
	}
	// Validate format of the ssh key
	if !strings.HasPrefix(sshKey, "----- BEGIN") {
		return true, fmt.Errorf("provided SSH key has invalid format")
	}

	// sftp:user@host:port//srv/repo
	s.url = "sftp:" + user + "@" + host + port + "/" + path

	// Give ssh client the ssh key to be able to connect to backup server passwordless
	done, err = s.propageteSshKey(sshKey)
	if err != nil || !done {
		return done, err
	}

	return true, nil
}

// propageteSshKey configures ssh client to use ssh key and trust the host
func (s *SftpServer) propageteSshKey(sshKey string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return true, fmt.Errorf("failed to get user home directory. Reason: %s", err.Error())
	}
	destDir := home + "/.ssh/"

	// Ensure destDir exists
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		err = os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return true, fmt.Errorf("failed to create SSH keys directory. Reason: %s", err.Error())
		}
	}

	// Save the key
	err = ioutil.WriteFile(destDir+"rest_rsa", []byte(sshKey), 0600)
	if err != nil {
		return true, fmt.Errorf("failed to propagate SSH key. Reason: %s", err.Error())
	}

	// Add backup server host to known_hosts

	// TODO rework this insecure approach
	// Do not check remote SSH server fingerprint when sending backup
	sshConfigPatch := "\nHost " + s.config.Hostname + "\n  StrictHostKeyChecking no\n"

	// Append config patch to ssh client config file or create it if doesn't exist
	sshConfigFile, err := os.OpenFile(destDir+"config", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return true, err
	}
	defer sshConfigFile.Close()
	if _, err := sshConfigFile.WriteString(sshConfigPatch); err != nil {
		return true, err
	}

	return true, nil
}
