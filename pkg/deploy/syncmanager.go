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
package deploy

import (
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Syncable interface {
	// Syncs object to a cluster.
	// Returns nil,nil if object successfully reconciled.
	Sync(ctx *DeployContext) (*reconcile.Result, error)
	// Does finalization (removes cluster scope objects, etc)
	Finalize(ctx *DeployContext) (*reconcile.Result, error)
	// Registration
	Register(sm *SyncManager)
}

type SyncManager struct {
	syncers      []Syncable
	failedSyncer Syncable
}

func NewSyncManager() *SyncManager {
	return &SyncManager{
		syncers:      make([]Syncable, 0),
		failedSyncer: nil,
	}
}

func (sm *SyncManager) RegisterSyncer(syncer Syncable) {
	sm.syncers = append(sm.syncers, syncer)
}

// Sync all objects consequently.
func (sm *SyncManager) SyncAll(ctx *DeployContext) (*reconcile.Result, error) {
	for _, syncer := range sm.syncers {
		result, err := syncer.Sync(ctx)
		if err != nil {
			sm.failedSyncer = syncer

			err = SetStatusDetails(ctx, InstallOrUpdateFailed, err.Error(), "")
			logrus.Errorf("Failed to update checluster status, cause: %v", err)
		} else if sm.failedSyncer == syncer {
			sm.failedSyncer = nil

			err = SetStatusDetails(ctx, "", "", "")
			if err != nil {
				return &reconcile.Result{Requeue: true}, err
			}
		}

		if result != nil {
			return result, err
		}
	}

	return nil, nil
}

func (sm *SyncManager) FinalizeAll(ctx *DeployContext) {
	for _, syncer := range sm.syncers {
		_, err := syncer.Finalize(ctx)
		if err != nil {
			logrus.Errorf("Failed to finalize, cause: %v", err)
		}
	}
}
