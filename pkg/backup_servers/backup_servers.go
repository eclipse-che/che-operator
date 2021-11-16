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
package backup_servers

import (
	"fmt"
	"reflect"
	"strings"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupServer represents functionality for backup servers
type BackupServer interface {
	// PrepareConfiguration validates and converts backup server configuration to internal format.
	// Does initialization if needed.
	PrepareConfiguration(client client.Client, namespace string) (bool, error)

	// InitRepository creates backup repository on the backup server side.
	InitRepository() (bool, error)

	// IsRepositoryExist check whether the repository alredy initialized.
	// Returns: exists, done, error
	IsRepositoryExist() (bool, bool, error)

	// CheckRepository verifies ability to connect to the remote backup server.
	// and checks credentials for correctness.
	CheckRepository() (bool, error)

	// SendSnapshot creates snapshot on the remote backup server from given directory.
	SendSnapshot(path string) (*SnapshotStat, bool, error)

	// DownloadLastSnapshot downloads newet snapshot from the remote backup server into given directory.
	DownloadLastSnapshot(path string) (bool, error)

	// DownloadSnapshot downloads specified snapshot from the remote backup server into given directory.
	DownloadSnapshot(snapshot string, path string) (bool, error)
}

// NewBackupServer is a factory to get backup server backend.
// Only one backup server is allowed at a time.
// Note, it is required to call PrepareConfiguration later in order to retrieve credentials and validate the server configuration.
func NewBackupServer(servers chev1.CheBackupServerConfigurationSpec) (BackupServer, error) {
	// Autodetect server type.
	// Only one backup server should be configured.
	serverType := ""
	count := 0
	// Iterate over all possible servers (fields of Spec.BackupServerConfig) to find non empty one(s)
	rv := reflect.ValueOf(servers)
	for i := 0; i < rv.NumField(); i++ {
		// Skip private fields
		if !rv.Field(i).CanInterface() {
			continue
		}
		// Check if actual field value is equal to the empty struct of the filed type
		value := rv.Field(i).Interface()
		zeroValue := reflect.Zero(rv.Field(i).Type()).Interface()
		if !reflect.DeepEqual(value, zeroValue) {
			// The server configuration is not empty.
			serverType = strings.ToLower(rv.Type().Field(i).Name)
			count++
		}
	}

	if count != 1 {
		if count == 0 {
			// No backup server is configured
			return nil, fmt.Errorf("at least one backup server should be configured")
		}
		// There are several servers configured
		return nil, fmt.Errorf("%d backup servers configured, but only one is allowed at a time", count)
	}

	var backupServer BackupServer
	switch serverType {
	case "rest":
		backupServer = &RestServer{config: servers.Rest}
	case "sftp":
		backupServer = &SftpServer{config: servers.Sftp}
	case "awss3":
		backupServer = &AwsS3Server{config: servers.AwsS3}
	default:
		// Should never happen
		return nil, fmt.Errorf("internal error while detecting backup server type")
	}

	return backupServer, nil
}
