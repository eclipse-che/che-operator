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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

// Reconcile all objects in a order they have been added
// If reconciliation failed then CheCluster status will be updated accordingly.
func (manager *ReconcileManager) ReconcileAll(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if err := AppendFinalizer(ctx, Finalizer); err != nil {
		return reconcile.Result{}, false, err
	}

	for _, reconciler := range manager.reconcilers {
		result, done, err := reconciler.Reconcile(ctx)
		if err != nil {
			manager.failedReconciler = reconciler
			reconcilerName := GetObjectType(reconciler)
			errMsg := fmt.Sprintf("Reconciler failed %s, cause: %v", reconcilerName, err)
			if err := SetStatusDetails(ctx, constants.InstallOrUpdateFailed, errMsg); err != nil {
				logrus.Errorf("Failed to update checluster status, cause: %v", err)
			}
		} else if manager.failedReconciler == reconciler {
			manager.failedReconciler = nil
			if err := SetStatusDetails(ctx, "", ""); err != nil {
				logrus.Errorf("Failed to update checluster status, cause: %v", err)
			}
		}

		if !done {
			return result, done, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (manager *ReconcileManager) FinalizeAll(ctx *chetypes.DeployContext) (done bool) {
	done = true
	for _, reconciler := range manager.reconcilers {
		if completed := reconciler.Finalize(ctx); !completed {
			reconcilerName := GetObjectType(reconciler)
			ctx.CheCluster.Status.Message = fmt.Sprintf("Finalization failed for reconciler: %s", reconcilerName)
			_ = UpdateCheCRStatus(ctx, "Message", ctx.CheCluster.Status.Message)

			done = false
		}
	}

	if done {
		if err := CleanUpAllFinalizers(ctx); err != nil {
			return false
		}
	}

	return done
}
