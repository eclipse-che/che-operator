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
	"fmt"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	backup "github.com/eclipse-che/che-operator/pkg/backup_servers"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
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

func NewRestoreContext(r *ReconcileCheClusterRestore, restoreCR *chev1.CheClusterRestore, backupServerConfig *chev1.CheBackupServerConfiguration) (*RestoreContext, error) {
	namespace := restoreCR.GetNamespace()

	backupServer, err := backup.NewBackupServer(backupServerConfig.Spec)
	if err != nil {
		return nil, err
	}

	cheCR, CRCount, err := util.FindCheClusterCRInNamespace(r.cachingClient, namespace)
	if err != nil {
		// Check if Che CR is present
		if CRCount > 0 {
			// Several instances present
			return nil, err
		}
		// Che is not deployed
		cheCR = nil
	}

	restoreState, err := NewRestoreState(restoreCR)
	if err != nil {
		return nil, err
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

// UpdateRestoreStatus updates stage message in CR status according to current restore phase
func (rctx *RestoreContext) UpdateRestoreStatus() error {
	phase := rctx.state.GetPhaseMessage()
	if phase != rctx.restoreCR.Status.Phase {
		rctx.restoreCR.Status.Phase = phase
		return rctx.r.UpdateCRStatus(rctx.restoreCR)
	}
	return nil
}

type RestoreState struct {
	backupDownloaded     bool
	oldCheCleaned        bool
	cheResourcesRestored bool
	cheCRRestored        bool
	cheAvailable         bool
	cheDatabaseRestored  bool
	cheRestored          bool
}

// RestoreState phase messages
// Each message represents state in progress, not done
const (
	restoreStateIn_backupDownloaded     = "Downloading backup from backup server"
	restoreStateIn_oldCheCleaned        = "Cleaning up existing Che"
	restoreStateIn_cheResourcesRestored = "Restoring Che related cluster objects"
	restoreStateIn_cheCRRestored        = "Restoring Che Custom Resource"
	restoreStateIn_cheAvailable         = "Waiting until clean Che is ready"
	restoreStateIn_cheDatabaseRestored  = "Restoring Che database"
	restoreStateIn_cheRestored          = "Waiting until Che is ready"
)

func (s *RestoreState) GetPhaseMessage() string {
	// Order of the checks below should comply with restore steps order
	if !s.backupDownloaded {
		return restoreStateIn_backupDownloaded
	}
	if !s.oldCheCleaned {
		return restoreStateIn_oldCheCleaned
	}
	if !s.cheResourcesRestored {
		return restoreStateIn_cheResourcesRestored
	}
	if !s.cheCRRestored {
		return restoreStateIn_cheCRRestored
	}
	if !s.cheAvailable {
		return restoreStateIn_cheAvailable
	}
	if !s.cheDatabaseRestored {
		return restoreStateIn_cheDatabaseRestored
	}
	if !s.cheRestored {
		return restoreStateIn_cheRestored
	}
	return ""
}

func NewRestoreState(restoreCR *chev1.CheClusterRestore) (*RestoreState, error) {
	rs := &RestoreState{}

	phase := restoreCR.Status.Phase
	if phase != "" {
		if restoreCR.Status.State == chev1.STATE_SUCCEEDED {
			phase = chev1.STATE_SUCCEEDED
		}
		switch phase {
		case chev1.STATE_SUCCEEDED:
			rs.cheRestored = true
			fallthrough
		case restoreStateIn_cheRestored:
			rs.cheDatabaseRestored = true
			fallthrough
		case restoreStateIn_cheDatabaseRestored:
			rs.cheAvailable = true
			fallthrough
		case restoreStateIn_cheAvailable:
			rs.cheCRRestored = true
			fallthrough
		case restoreStateIn_cheCRRestored:
			rs.cheResourcesRestored = true
			fallthrough
		case restoreStateIn_cheResourcesRestored:
			rs.oldCheCleaned = true
			fallthrough
		case restoreStateIn_oldCheCleaned:
			rs.backupDownloaded = true
			fallthrough
		case restoreStateIn_backupDownloaded:
			break
		default:
			return nil, fmt.Errorf("unrecognized restore phase '%s' in status", phase)
		}
	}

	logrus.Debugf("Restore state: %v", rs)
	return rs, nil
}
