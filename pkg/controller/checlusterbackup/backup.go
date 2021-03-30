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
	"github.com/sirupsen/logrus"
)

const (
	// localDestDir specifies directory into which files for backup should be gathered and then send to specified backup server
	localDestDir = "/tmp/che-backup"
)

// CreateBackup gathers Che data and sends to backup server
func (r *ReconcileCheClusterBackup) CreateBackup(backupCR *orgv1.CheClusterBackup) (err error) {
	err = r.collectCheData(localDestDir)
	if err != nil {
		logrus.Error("Failed to collect backup data", err)
		return err
	}

	err = r.sendBackup(backupCR)
	if err != nil {
		logrus.Error("Failed to send data to backup server", err)
		return err
	}

	return nil
}

func (r *ReconcileCheClusterBackup) collectCheData(backupDir string) error {
	return nil
}

func (r *ReconcileCheClusterBackup) sendBackup(backupCR *orgv1.CheClusterBackup) error {
	// TODO read additional CAs and merge with system's bundle
	return nil
}
