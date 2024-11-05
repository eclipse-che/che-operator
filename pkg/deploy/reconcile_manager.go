//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	reconcilerLogger = ctrl.Log.WithName("reconciler_manager")
)

const Finalizer = "cluster-resources." + constants.FinalizerSuffix

type Reconcilable interface {
	// Reconcile object.
	Reconcile(ctx *chetypes.DeployContext) (result reconcile.Result, done bool, err error)
	// Does finalization (removes cluster scope objects, etc)
	Finalize(ctx *chetypes.DeployContext) (done bool)
}

type ReconcileManager struct {
	reconcilers      []Reconcilable
	failedReconciler Reconcilable
}

func NewReconcileManager() *ReconcileManager {
	return &ReconcileManager{
		reconcilers:      make([]Reconcilable, 0),
		failedReconciler: nil,
	}
}

func (manager *ReconcileManager) RegisterReconciler(reconciler Reconcilable) {
	manager.reconcilers = append(manager.reconcilers, reconciler)
}

// ReconcileAll reconciles all objects in an order they have been added.
// If reconciliation failed then CheCluster status will be updated accordingly.
func (manager *ReconcileManager) ReconcileAll(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if err := AppendFinalizer(ctx, Finalizer); err != nil {
		return reconcile.Result{}, false, err
	}

	for _, reconciler := range manager.reconcilers {
		reconcilerName := GetObjectType(reconciler)

		//reconcilerLogger.Info("Reconciling started", "reconciler", reconcilerName)
		result, done, err := reconciler.Reconcile(ctx)
		//reconcilerLogger.Info("Reconciled completed", "reconciler", reconcilerName, "done", done)

		if err != nil {
			// set failed reconciler
			manager.failedReconciler = reconciler

			errMsg := fmt.Sprintf("Reconciler failed %s, cause: %v", reconcilerName, err)
			if err := SetStatusDetails(ctx, constants.InstallOrUpdateFailed, errMsg); err != nil {
				reconcilerLogger.Error(err, "Failed to update checluster status")
			}
		} else if manager.failedReconciler == reconciler {
			// cleanup failed reconciler
			manager.failedReconciler = nil

			if err := SetStatusDetails(ctx, "", ""); err != nil {
				reconcilerLogger.Error(err, "Failed to update checluster status")
			}
		}

		// don't continue if reconciliation failed
		if !done {
			return result, done, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (manager *ReconcileManager) FinalizeAll(ctx *chetypes.DeployContext) (done bool) {
	done = true
	for _, reconciler := range manager.reconcilers {
		completed := reconciler.Finalize(ctx)
		done = done && completed

		if !completed {
			// don't prevent from invoking other finalizers, just log the error
			reconcilerLogger.Error(nil, fmt.Sprintf("Finalization failed for reconciler: %s", GetObjectType(reconciler)))
		}
	}

	if done {
		// Removes remaining finalizers not to prevent CheCluster object from being deleted
		if err := CleanUpAllFinalizers(ctx); err != nil {
			return false
		}
	}

	return done
}
