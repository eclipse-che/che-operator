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
package checlusterbackup

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// Common interface for backup servers
type BackupServer interface {
	// Checks whether all mandatory configuration fields are filled and has valid format.
	// Returns done status and error if any.
	// If done is true and there is an error, it means that the configuration is not valid.
	ValidateConfiguration(bctx *BackupContext) (bool, error)
}

// List of backup servers

type RestServer struct {
	config orgv1.RestServerConfing
}

type SftpServer struct {
	config orgv1.SftpServerConfing
}

type AwsS3Server struct {
	config orgv1.AwsS3ServerConfig
}

// GetCurrentBackupServer is factory to get current backup server backend.
// Returns error if the backup server type is not properly configured.
func GetCurrentBackupServer(backupCR *orgv1.CheClusterBackup) (BackupServer, error) {
	var backupServer BackupServer

	rv := reflect.ValueOf(backupCR.Spec.Servers)
	if backupCR.Spec.ServerType != "" {
		// CR provides backup server to use
		serverType := strings.ToLower(backupCR.Spec.ServerType)

		// Iterate over all possible servers (fields of Spec.Servers) to find specified one
		for i := 0; i < rv.NumField(); i++ {
			// Skip private fields
			if !rv.Field(i).CanInterface() {
				continue
			}
			if serverType == strings.ToLower(rv.Type().Field(i).Name) {
				// Found specified backend server.
				// It is safe to cast here as all backup servers implement BackupServer interface.
				backupServer = rv.Field(i).Interface().(BackupServer)
				return backupServer, nil
			}
		}

		// Given backup server not found.
		// User gave illegal server in the Spec.ServerType filed of the CR.
		return nil, fmt.Errorf("unrecognized backup server type '%s'", serverType)
	}

	// No backup server configuration is specified in the CR,
	// so only one backup server should be configured.
	count := 0
	// Iterate over all possible servers (fields of Spec.Servers) to find non empty one(s)
	for i := 0; i < rv.NumField(); i++ {
		// Skip private fields
		if !rv.Field(i).CanInterface() {
			continue
		}
		// Check if actual field value is equal to the empty struct of the filed type
		if rv.Field(i).Interface() != reflect.New(rv.Field(i).Type()) {
			// The server configuration is not empty.
			// It is safe to cast here as all backup servers implement BackupServer interface.
			backupServer = rv.Field(i).Interface().(BackupServer)
			count++
		}
	}

	if count != 1 {
		if count == 0 {
			// No server is configured
			return nil, fmt.Errorf("at least one backup server should be configured")
		}
		// There are several servers configured, but it is not specified which server to use
		return nil, fmt.Errorf("%d backup servers configured, please select which one to use by setting 'ServerType' field", count)
	}

	return backupServer, nil
}

// Backup server type specific implementation

// Validation

// validateResticRepoPassword check whethwer a password specified.
// It doesn't check the password correctness.
func validateResticRepoPassword(bctx *BackupContext, rp orgv1.RepoPassword) (bool, error) {
	if rp.RepoPassword != "" {
		return true, nil
	}

	if rp.RepoPasswordSecretRef == "" {
		return true, fmt.Errorf("restic repository password should be specified")
	}
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: bctx.backupCR.GetNamespace(), Name: rp.RepoPasswordSecretRef}
	err := bctx.r.client.Get(context.TODO(), namespacedName, secret)
	if err == nil {
		if _, exist := secret.Data["repo-password"]; !exist {
			return true, fmt.Errorf("%s secret should have 'repo-password' field", rp.RepoPasswordSecretRef)
		}
	} else if !errors.IsNotFound(err) {
		return false, err
	}
	return true, fmt.Errorf("secret '%s' with restic repository password not found", rp.RepoPasswordSecretRef)
}

func (s *RestServer) ValidateConfiguration(bctx *BackupContext) (bool, error) {
	done, err := validateResticRepoPassword(bctx, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}

	if s.config.Protocol != "" && !(s.config.Protocol == "http" || s.config.Protocol == "https") {
		return true, fmt.Errorf("unrecognized protocol %s for REST server", s.config.Protocol)
	}
	if s.config.Hostname == "" {
		return true, fmt.Errorf("REST server hostname must be configured")
	}

	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: bctx.backupCR.GetNamespace(), Name: s.config.CredentialsSecretRef}
	err = bctx.r.client.Get(context.TODO(), namespacedName, secret)
	if err == nil {
		// Check the secret fields
		if _, exist := secret.Data["username"]; !exist {
			return true, fmt.Errorf("%s secret should have 'username' field", secret.ObjectMeta.Name)
		}
		if _, exist := secret.Data["password"]; !exist {
			return true, fmt.Errorf("%s secret should have 'password' field", secret.ObjectMeta.Name)
		}
	} else if !errors.IsNotFound(err) {
		return false, err
	}

	return true, nil
}

func (s *SftpServer) ValidateConfiguration(bctx *BackupContext) (bool, error) {
	done, err := validateResticRepoPassword(bctx, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}

	if s.config.Username == "" {
		return true, fmt.Errorf("SFTP server username must be configured")
	}
	if s.config.Hostname == "" {
		return true, fmt.Errorf("SFTP server hostname must be configured")
	}

	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: bctx.backupCR.GetNamespace(), Name: s.config.SshKeySecretRef}
	if err := bctx.r.client.Get(context.TODO(), namespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			return true, fmt.Errorf("SSH key is mandatory to connect to SFTP backup server")
		}
		return false, err
	}

	return true, nil
}

func (s *AwsS3Server) ValidateConfiguration(bctx *BackupContext) (bool, error) {
	done, err := validateResticRepoPassword(bctx, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}

	// TODO
	return true, nil
}
