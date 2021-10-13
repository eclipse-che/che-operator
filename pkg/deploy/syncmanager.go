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
	"reflect"
	"runtime"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconcilable interface {
	// Reconcile object.
	Reconcile(ctx *DeployContext) (result reconcile.Result, done bool, err error)
	// Does finalization (removes cluster scope objects, etc)
	Finalize(ctx *DeployContext) (done bool, err error)
	// Registration
	Register(reconcileManager *ReconcileManager)
}

type ReconcileManager struct {
	reconcilers      []Reconcilable
	failedReconciler *Reconcilable
}

func NewReconcileManager() *ReconcileManager {
	return &ReconcileManager{
		reconcilers:      make([]Reconcilable, 0),
		failedReconciler: nil,
	}
}

func (rm *ReconcileManager) RegisterReconciler(reconciler Reconcilable) {
	rm.reconcilers = append(rm.reconcilers, reconciler)
}

// Reconcile all objects in a order they have been added
// If reconciliation failed then CheCluster status will be updated accordingly.
func (rm *ReconcileManager) ReconcileAll(ctx *DeployContext) (reconcile.Result, bool, error) {
	for _, reconciler := range rm.reconcilers {
		result, done, err := reconciler.Reconcile(ctx)
		if err != nil {
			rm.failedReconciler = &reconciler
			if err := SetStatusDetails(ctx, InstallOrUpdateFailed, err.Error(), ""); err != nil {
				logrus.Errorf("Failed to update checluster status, cause: %v", err)
			}
		} else if rm.failedReconciler == &reconciler {
			rm.failedReconciler = nil
			if err := SetStatusDetails(ctx, "", "", ""); err != nil {
				logrus.Errorf("Failed to update checluster status, cause: %v", err)
			}
		}

		if !done {
			return result, done, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (sm *ReconcileManager) FinalizeAll(ctx *DeployContext) {
	for _, reconciler := range sm.reconcilers {
		_, err := reconciler.Finalize(ctx)
		if err != nil {
			reconcilerName := runtime.FuncForPC(reflect.ValueOf(reconciler).Pointer()).Name()
			logrus.Errorf("Finalization failed for reconciler: `%s`, cause: %v", reconcilerName, err)
		}
	}
}
