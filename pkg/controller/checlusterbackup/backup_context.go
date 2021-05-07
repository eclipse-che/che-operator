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
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	backup "github.com/eclipse-che/che-operator/pkg/backup_servers"
	"github.com/eclipse-che/che-operator/pkg/util"
)

type BackupContext struct {
	namespace    string
	r            *ReconcileCheClusterBackup
	backupCR     *orgv1.CheClusterBackup
	cheCR        *orgv1.CheCluster
	backupServer backup.BackupServer
}

func NewBackupContext(r *ReconcileCheClusterBackup, backupCR *orgv1.CheClusterBackup) (*BackupContext, error) {
	namespace := backupCR.GetNamespace()

	backupServer, err := backup.NewBackupServer(backupCR.Spec.Servers, backupCR.Spec.ServerType)
	if err != nil {
		// Allow no backup servers configured if internal backup server is requested
		if !(backupCR.Spec.AutoconfigureRestBackupServer && strings.HasPrefix(err.Error(), "at least one")) {
			return nil, err
		}
		// backupServer is nil, because no backup server has been configured.
		// Also, AutoconfigureRestBackupServer property set to true, so
		// the configuration will be added and the server set up automatically by the operator.
		// After the preparations, a new reconcile loop will be triggered, so backupServer will not be nil any more.
	}

	cheCR, _, err := util.FindCheCRinNamespace(r.client, namespace)
	if err != nil {
		return nil, err
	}

	return &BackupContext{
		namespace:    namespace,
		r:            r,
		backupCR:     backupCR,
		cheCR:        cheCR,
		backupServer: backupServer,
	}, nil
}
