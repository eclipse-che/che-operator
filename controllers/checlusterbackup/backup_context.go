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
	"fmt"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	backup "github.com/eclipse-che/che-operator/pkg/backup_servers"
	"github.com/eclipse-che/che-operator/pkg/util"
)

type BackupContext struct {
	namespace            string
	r                    *ReconcileCheClusterBackup
	backupCR             *chev1.CheClusterBackup
	cheCR                *chev1.CheCluster
	backupServerConfigCR *chev1.CheBackupServerConfiguration
	backupServer         backup.BackupServer
	state                *BackupState
}

func NewBackupContext(r *ReconcileCheClusterBackup, backupCR *chev1.CheClusterBackup, backupServerConfig *chev1.CheBackupServerConfiguration) (backupContext *BackupContext, err error) {
	namespace := backupCR.GetNamespace()

	var backupServer backup.BackupServer
	if backupServerConfig != nil {
		backupServer, err = backup.NewBackupServer(backupServerConfig.Spec)
		if err != nil {
			return nil, err
		}
	} else {
		if !backupCR.Spec.UseInternalBackupServer {
			return nil, fmt.Errorf("no backup configuration given")
		}
		// backupServer is nil, because no backup server has been configured.
		// Also, UseInternalBackupServer property is set to true, so
		// the configuration will be added and the server set up automatically by the operator.
		// After the preparations, a new reconcile loop will be triggered, so backupServer will not be nil any more.
	}

	cheCR, _, err := util.FindCheClusterCRInNamespace(r.cachingClient, namespace)
	if err != nil {
		return nil, err
	}

	backupState, err := NewBackupState(backupCR)
	if err != nil {
		return nil, err
	}

	backupContext = &BackupContext{
		namespace:            namespace,
		r:                    r,
		backupCR:             backupCR,
		cheCR:                cheCR,
		backupServerConfigCR: backupServerConfig,
		backupServer:         backupServer,
		state:                backupState,
	}
	return backupContext, nil
}

func (bctx *BackupContext) UpdateBackupStatusPhase() error {
	phase := bctx.state.GetPhaseMessage()
	if phase != bctx.backupCR.Status.Phase {
		bctx.backupCR.Status.Phase = phase
		return bctx.r.UpdateCRStatus(bctx.backupCR)
	}
	return nil
}

// BackupState represents current step in backup process
// Note, that reconcile loop is considered stateless, so
// the state should be inferred from CR (mainly from status)
type BackupState struct {
	internalBackupServerSetup          bool
	backupRepositoryReady              bool
	cheInstallationBackupDataCollected bool
	backupSnapshotSent                 bool
}

// BackupState phase messages
// Each message represents state in progress, not done
const (
	backupStateIn_internalBackupServerSetup          = "Setting up internal backup server"
	backupStateIn_backupRepositoryReady              = "Connecting to backup repository"
	backupStateIn_cheInstallationBackupDataCollected = "Collecting Che installation data"
	backupStateIn_backupSnapshotSent                 = "Sending backup shapshot to backup server"
)

// GetPhaseMessage returns message that describes action in progress now
func (s *BackupState) GetPhaseMessage() string {
	if !s.internalBackupServerSetup {
		return backupStateIn_internalBackupServerSetup
	}
	if !s.backupRepositoryReady {
		return backupStateIn_backupRepositoryReady
	}
	if !s.cheInstallationBackupDataCollected {
		return backupStateIn_cheInstallationBackupDataCollected
	}
	if !s.backupSnapshotSent {
		return backupStateIn_backupSnapshotSent
	}
	return ""
}

func NewBackupState(backupCR *chev1.CheClusterBackup) (*BackupState, error) {
	bs := &BackupState{}

	phase := backupCR.Status.Phase
	if phase != "" {
		if backupCR.Status.State == chev1.STATE_SUCCEEDED {
			phase = chev1.STATE_SUCCEEDED
		}
		switch phase {
		case chev1.STATE_SUCCEEDED:
			bs.backupSnapshotSent = true
			fallthrough
		case backupStateIn_backupSnapshotSent:
			bs.cheInstallationBackupDataCollected = true
			fallthrough
		case backupStateIn_cheInstallationBackupDataCollected:
			bs.backupRepositoryReady = true
			fallthrough
		case backupStateIn_backupRepositoryReady:
			bs.internalBackupServerSetup = true
			fallthrough
		case backupStateIn_internalBackupServerSetup:
			break
		default:
			return nil, fmt.Errorf("unrecognized backup phase '%s' in status", phase)
		}
	}

	if !backupCR.Spec.UseInternalBackupServer {
		// If there is no request for internal backup server, consider this step as completed
		bs.internalBackupServerSetup = true
	}

	return bs, nil
}
