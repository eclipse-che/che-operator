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
	chev1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	backup "github.com/eclipse-che/che-operator/pkg/backup_servers"
	"github.com/eclipse-che/che-operator/pkg/util"
)

type RestoreContext struct {
	namespace    string
	r            *ReconcileCheClusterRestore
	restoreCR    *chev1.CheClusterRestore
	cheCR        *chev1.CheCluster
	backupServer backup.BackupServer
	state        *RestoreState
	isOpenShift  bool
}

func NewRestoreContext(r *ReconcileCheClusterRestore, restoreCR *chev1.CheClusterRestore) (*RestoreContext, error) {
	namespace := restoreCR.GetNamespace()

	backupServer, err := backup.NewBackupServer(restoreCR.Spec.BackupServerConfig)
	if err != nil {
		return nil, err
	}

	cheCR, CRCount, err := util.FindCheCRinNamespace(r.client, namespace)
	if err != nil {
		// Check if Che CR is present
		if CRCount > 0 {
			// Several instances present
			return nil, err
		}
		// Che is not deployed
		cheCR = nil
	}

	isOpenShift, _, _ := util.DetectOpenShift()

	return &RestoreContext{
		namespace:    namespace,
		r:            r,
		restoreCR:    restoreCR,
		cheCR:        cheCR,
		backupServer: backupServer,
		state:        restoreState,
		isOpenShift:  isOpenShift,
	}, nil
}

// UpdateRestoreStatus updates stage message in CR status according to current restore phase.
// Needed only to show progress to the user.
func (rctx *RestoreContext) UpdateRestoreStatus() error {
	rctx.restoreCR.Status.Phase = rctx.state.GetProgressMessage()
	rctx.restoreCR.Status.State = chev1.STATE_IN_PROGRESS
	if rctx.restoreCR.Status.Phase != "" {
		rctx.restoreCR.Status.Message = "Che is being restored"
	} else {
		rctx.restoreCR.Status.Message = ""
	}
	return rctx.r.UpdateCRStatus(rctx.restoreCR)
}

// Keep state as a global variable to preserve between reconcile loops
var restoreState = NewRestoreState()

type RestoreState struct {
	backupDownloaded     bool
	oldCheCleaned        bool
	cheResourcesRestored bool
	cheCRRestored        bool
	cheAvailable         bool
	cheDatabaseRestored  bool
	cheRestored          bool
}

func (rs *RestoreState) Reset() {
	rs.backupDownloaded = false
	rs.oldCheCleaned = false
	rs.cheResourcesRestored = false
	rs.cheCRRestored = false
	rs.cheAvailable = false
	rs.cheDatabaseRestored = false
	rs.cheRestored = false
}

func NewRestoreState() *RestoreState {
	rs := &RestoreState{}
	rs.Reset()
	return rs
}

func (s *RestoreState) GetProgressMessage() string {
	// Order of the checks below should comply with restore steps order in
	// RestoreChe function, except backup downloading step that precedes it.
	if !s.backupDownloaded {
		return "Downloading backup from backup server"
	}
	if !s.oldCheCleaned {
		return "Cleaning up existing Che"
	}
	if !s.cheResourcesRestored {
		return "Restoring Che related cluster objects"
	}
	if !s.cheCRRestored {
		return "Restoring Che Custom Resource"
	}
	if !s.cheAvailable {
		return "Waiting until Che is ready"
	}
	if !s.cheDatabaseRestored {
		return "Restoring Che database"
	}
	if !s.cheRestored {
		return "Waiting until Che is ready"
	}
	return ""
}
