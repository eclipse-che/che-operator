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

func (r *ReconcileCheClusterBackup) ValidateCurrentBackupServerSettings(backupCR *orgv1.CheClusterBackup) (bool, error) {
	// Check current backup server settings
	var currentBackupServer string
	if backupCR.Spec.ServerType == "" {
		// Only one backup server should be configured
		var n int
		n, currentBackupServer = countConfiguredBackupServers(backupCR)
		if n != 1 {
			if n == 0 {
				return true, fmt.Errorf("at least one backup server should be configured")
			}
			return true, fmt.Errorf("%d backup servers configured, please select which one to use by setting 'ServerType' field", n)
		}
	} else {
		currentBackupServer = backupCR.Spec.ServerType

		backupServerTypeValid := false
		for _, value := range orgv1.ServerTypes() {
			if value == currentBackupServer {
				backupServerTypeValid = true
				break
			}
		}
		if !backupServerTypeValid {
			return true, fmt.Errorf("unrecognized backup server type '%s'", currentBackupServer)
		}
	}

	if done, err := r.validateBackupServer(backupCR, currentBackupServer); err != nil {
		return done, err
	}

	return true, nil
}

// validateBackupServer validates configuration of a backup server.
// serverType should be name of a field of BackupServers struct
func (r *ReconcileCheClusterBackup) validateBackupServer(backupCR *orgv1.CheClusterBackup, serverType string) (bool, error) {
	serverType = strings.ToLower(serverType)
	switch serverType {
	case orgv1.SERVER_TYPE_INTERNAL:
		return r.validateRestServerConfig(backupCR.Spec.Servers.Internal, backupCR.GetNamespace())
	case orgv1.SERVER_TYPE_REST:
		return r.validateRestServerConfig(backupCR.Spec.Servers.Rest, backupCR.GetNamespace())
	case orgv1.SERVER_TYPE_SFTP:
		return r.validateSftpServerConfig(backupCR.Spec.Servers.Sftp, backupCR.GetNamespace())
	case orgv1.SERVER_TYPE_AWSS3:
		return r.validateAwsS3ServerConfig(backupCR.Spec.Servers.AwsS3, backupCR.GetNamespace())
	case orgv1.SERVER_TYPE_MINIO:
		return r.validateAwsS3ServerConfig(backupCR.Spec.Servers.Minio, backupCR.GetNamespace())
	default:
		return true, fmt.Errorf("unrecognized backup server type '%s'", serverType)
	}
}

// countConfiguredBackupServers counts non-empty beckup servers configured.
// If only one server is configured, then its type (e.g. sftp, rest, awss3) is returned.
func countConfiguredBackupServers(backupCR *orgv1.CheClusterBackup) (int, string) {
	n := 0
	serverType := ""

	rv := reflect.ValueOf(backupCR.Spec.Servers)
	for i := 0; i < rv.NumField(); i++ {
		// Skip private fields
		if !rv.Field(i).CanInterface() {
			continue
		}
		// Check if actual field value is equal to the empty struct of the filed type
		if rv.Field(i).Interface() == reflect.New(rv.Field(i).Type()) {
			n++
			// Server type is the filed name
			serverType = strings.ToLower(rv.Type().Field(i).Name)
		}
	}

	if n != 1 {
		serverType = ""
	}
	return n, serverType
}

// validateResticRepoPassword check whethwer a password specified.
// It doesn't check the password correctness.
func (r *ReconcileCheClusterBackup) validateResticRepoPassword(rp orgv1.RepoPassword, namespace string) (bool, error) {
	if rp.RepoPassword != "" {
		return true, nil
	}

	if rp.RepoPasswordSecretRef == "" {
		return true, fmt.Errorf("restic repository password should be specified")
	}
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: rp.RepoPasswordSecretRef}
	err := r.client.Get(context.TODO(), namespacedName, secret)
	if err == nil {
		if _, exist := secret.Data["repo-password"]; !exist {
			return true, fmt.Errorf("%s secret should have 'repo-password' field", rp.RepoPasswordSecretRef)
		}
	} else if !errors.IsNotFound(err) {
		return false, err
	}
	return true, fmt.Errorf("secret '%s' with restic repository password not found", rp.RepoPasswordSecretRef)
}

func (r *ReconcileCheClusterBackup) validateRestServerConfig(config orgv1.RestServerConfing, namespace string) (bool, error) {
	done, err := r.validateResticRepoPassword(config.RepoPassword, namespace)
	if err != nil || !done {
		return done, err
	}

	if config.Protocol != "" && !(config.Protocol == "http" || config.Protocol == "https") {
		return true, fmt.Errorf("unrecognized protocol %s for REST server", config.Protocol)
	}
	if config.Hostname == "" {
		return true, fmt.Errorf("REST server hostname must be configured")
	}

	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: config.CredentialsSecretRef}
	err = r.client.Get(context.TODO(), namespacedName, secret)
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

func (r *ReconcileCheClusterBackup) validateSftpServerConfig(config orgv1.SftpServerConfing, namespace string) (bool, error) {
	done, err := r.validateResticRepoPassword(config.RepoPassword, namespace)
	if err != nil || !done {
		return done, err
	}

	if config.Username == "" {
		return true, fmt.Errorf("SFTP server username must be configured")
	}
	if config.Hostname == "" {
		return true, fmt.Errorf("SFTP server hostname must be configured")
	}

	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: config.SshKeySecretRef}
	if err := r.client.Get(context.TODO(), namespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			return true, fmt.Errorf("SSH key is mandatory to connect to SFTP backup server")
		}
		return false, err
	}

	return true, nil
}

func (r *ReconcileCheClusterBackup) validateAwsS3ServerConfig(config orgv1.AwsS3ServerConfig, namespace string) (bool, error) {
	done, err := r.validateResticRepoPassword(config.RepoPassword, namespace)
	if err != nil || !done {
		return done, err
	}

	// TODO
	return true, nil
}

// func countBackupServersConfigs(backupCR *orgv1.CheClusterBackup) (int, string) {
// 	Servers := backupCR.Spec.Servers
// 	n := 0
// 	serverType := ""
// 	if Servers.Internal != (orgv1.RestServerConfing{}) {
// 		n++
// 		serverType = orgv1.SERVER_TYPE_INTERNAL
// 	}
// 	if Servers.Sftp != (orgv1.SftpServerConfing{}) {
// 		n++
// 		serverType = orgv1.SERVER_TYPE_SFTP
// 	}
// 	if Servers.Rest != (orgv1.RestServerConfing{}) {
// 		n++
// 		serverType = orgv1.SERVER_TYPE_REST
// 	}
// 	if Servers.AwsS3 != (orgv1.AwsS3ServerConfig{}) {
// 		n++
// 		serverType = orgv1.SERVER_TYPE_AWSS3
// 	}
// 	if Servers.Minio != (orgv1.AwsS3ServerConfig{}) {
// 		n++
// 		serverType = orgv1.SERVER_TYPE_MINIO
// 	}
//
// 	if n != 1 {
// 		serverType = ""
// 	}
// 	return n, serverType
// }
