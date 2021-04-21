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
	"strings"

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
	state        RestoreState
}

func NewRestoreContext(r *ReconcileCheClusterRestore, restoreCR *orgv1.CheClusterRestore) (*RestoreContext, error) {
	namespace := restoreCR.GetNamespace()

	backupServer, err := backup.NewBackupServer(restoreCR.Spec.Servers, restoreCR.Spec.ServerType)
	if err != nil {
		return nil, err
	}

	cheCR, err := util.FindCheCRinNamespace(r.client, namespace)
	if err != nil {
		// Check if Che CR is present
		// TODO find better solution
		if !strings.HasSuffix(err.Error(), "got 0 instances") {
			return nil, err
		}
		// Che is not deployed
		cheCR = nil
	}

	return &RestoreContext{
		namespace:    namespace,
		r:            r,
		restoreCR:    restoreCR,
		cheCR:        cheCR,
		backupServer: backupServer,
		state:        restoreState,
	}, nil
}

// UpdateRestoreStage updates stage message in CR status according to current restore phase.
// Needed only to show progress to the user.
func (rctx *RestoreContext) UpdateRestoreStage() error {
	rctx.restoreCR.Status.Stage = rctx.state.GetProgressMessage()
	return rctx.r.UpdateCRStatus(rctx.restoreCR)
}

// Keep state as a global variable to preserve between reconcile loops
var restoreState = NewRestoreState()

type RestoreState struct {
	backupDownloaded     bool
	oldCheAvailable      bool
	oldCheSuspended      bool
	cheResourcesRestored bool
	cheDatabaseRestored  bool
	cheCRRestored        bool
	cheRestored          bool
}

func NewRestoreState() RestoreState {
	return RestoreState{
		backupDownloaded:     false,
		oldCheAvailable:      false,
		oldCheSuspended:      false,
		cheResourcesRestored: false,
		cheDatabaseRestored:  false,
		cheCRRestored:        false,
		cheRestored:          false,
	}
}

func (s RestoreState) GetProgressMessage() string {
	if !s.backupDownloaded {
		return "Downloading backup from backup server"
	}
	if !s.oldCheAvailable {
		return "Deploying clean Che"
	}
	if !s.oldCheSuspended {
		return "Suspending existing Che"
	}
	if !s.cheResourcesRestored {
		return "Restoring Che related cluster objects"
	}
	if !s.cheDatabaseRestored {
		return "Restoring Che database"
	}
	if !s.cheCRRestored {
		return "Restoring Che Custom Resource"
	}
	if !s.cheRestored {
		return "Waiting until Che is ready"
	}
	return ""
}
