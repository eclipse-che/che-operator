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
package checlusterrestore

import (
	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	backup "github.com/eclipse-che/che-operator/pkg/backup_servers"
	"github.com/eclipse-che/che-operator/pkg/util"
)

type RestoreContext struct {
	namespace    string
	r            *ReconcileCheClusterRestore
	restoreCR    *orgv1.CheClusterRestore
	cheCR        *orgv1.CheCluster
	backupServer backup.BackupServer
}

func NewRestoreContext(r *ReconcileCheClusterRestore, restoreCR *orgv1.CheClusterRestore) (*RestoreContext, error) {
	namespace := restoreCR.GetNamespace()

	backupServer, err := backup.NewBackupServer(restoreCR.Spec.Servers, restoreCR.Spec.ServerType)
	if err != nil {
		return nil, err
	}

	cheCR, err := util.FindCheCRinNamespace(r.client, namespace)
	if err != nil {
		return nil, err
	}

	return &RestoreContext{
		namespace:    namespace,
		r:            r,
		restoreCR:    restoreCR,
		cheCR:        cheCR,
		backupServer: backupServer,
	}, nil
}
