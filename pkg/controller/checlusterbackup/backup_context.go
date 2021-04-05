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
	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
)

type BackupContext struct {
	r            *ReconcileCheClusterBackup
	backupCR     *orgv1.CheClusterBackup
	backupServer BackupServer
	optional     backupContextOptional
}

type backupContextOptional struct {
	cheCR *orgv1.CheCluster
}

func NewBackupContext(r *ReconcileCheClusterBackup, backupCR *orgv1.CheClusterBackup) (*BackupContext, error) {
	backupServer, err := GetCurrentBackupServer(backupCR)
	if err != nil {
		return nil, err
	}

	return &BackupContext{
		r:            r,
		backupCR:     backupCR,
		backupServer: backupServer,
		optional:     backupContextOptional{},
	}, nil
}
