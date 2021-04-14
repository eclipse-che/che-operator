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
	"fmt"
	"reflect"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupServer represents functionality for backup servers
type BackupServer interface {
	// PrepareConfiguration validates and converts backup server configuration to internal format.
	// Does initialization if needed.
	PrepareConfiguration(client client.Client, namespace string) (bool, error)

	// InitRepository creates backup repository on the backup server side.
	InitRepository() (bool, error)

	// IsRepositoryExist check whether the repository alredy innitialized.
	IsRepositoryExist() (bool, bool, error)

	// CheckRepository verifies ability to connect to the remote backup server.
	// and checks credentials for correctness.
	CheckRepository() (bool, error)

	// SendSnapshot creates snapshot on the remote backup server from given directory.
	SendSnapshot(path string) (bool, error)

	// DownloadLastSnapshot downloads newet snapshot from the remote backup server into given directory.
	DownloadLastSnapshot(path string) (bool, error)

	// DownloadSnapshot downloads specified snapshot from the remote backup server into given directory.
	DownloadSnapshot(snapshot string, path string) (bool, error)
}

// NewBackupServer is a factory to get backup server backend.
// If only one backup server is configured, serverType can be empty.
// Note, it is required to call PrepareConfiguration later in order to retrieve and validate the server configuration.
// Returns error if the backup server type is invalid or not properly configured.
func NewBackupServer(servers orgv1.BackupServers, serverType string) (BackupServer, error) {
	var backupServer BackupServer

	if serverType == "" {
		// Autodetect server type.
		// Only one backup server should be configured.
		count := 0
		// Iterate over all possible servers (fields of Spec.Servers) to find non empty one(s)
		rv := reflect.ValueOf(servers)
		for i := 0; i < rv.NumField(); i++ {
			// Skip private fields
			if !rv.Field(i).CanInterface() {
				continue
			}
			// Check if actual field value is equal to the empty struct of the filed type
			if rv.Field(i).Interface() != reflect.New(rv.Field(i).Type()) {
				// The server configuration is not empty.
				serverType = strings.ToLower(rv.Type().Field(i).Name)
				count++
			}
		}

		if count != 1 {
			if count == 0 {
				// No server is configured
				return nil, fmt.Errorf("at least one backup server should be configured")
			}
			// There are several servers configured, but it is not specified which server to use
			return nil, fmt.Errorf("%d backup servers configured, please select which one to use by setting 'serverType' field", count)
		}
	}

	switch serverType {
	case "internal":
		backupServer = &RestServer{Config: servers.Internal}
	case "rest":
		backupServer = &RestServer{Config: servers.Rest}
	case "sftp":
		backupServer = &SftpServer{config: servers.Sftp}
	case "awss3":
		backupServer = &AwsS3Server{config: servers.AwsS3}
	default:
		return nil, fmt.Errorf("unknown server type: %s, please correct spec.serverType field", serverType)
	}

	return backupServer, nil
}
