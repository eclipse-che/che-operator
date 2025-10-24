//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package reconciler

import (
	"fmt"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconcilable defines the interface for components that can be reconciled and finalized.
type Reconcilable interface {
	// Reconcile performs a reconciliation step to ensure the desired state matches the actual state.
	// Returns:
	// - result: reconcile.Result indicating when to requeue (if needed)
	// - done: true if reconciliation completed successfully, false if it needs to be retried
	// - err: any error encountered during reconciliation
	Reconcile(ctx *chetypes.DeployContext) (result reconcile.Result, done bool, err error)

	// Finalize performs cleanup operations before the resource is deleted.
	// All created resources should be removed, including:
	// - finalizers
	// - cluster scoped resources
	// Returns true if finalization completed successfully, false otherwise.
	// TODO make Finalize return error as well
	Finalize(ctx *chetypes.DeployContext) (done bool)
}

// ReconcilerManager manages a collection of Reconcilable objects and executes them in order.
type ReconcilerManager struct {
	reconcilers []Reconcilable
}

func NewReconcilerManager() *ReconcilerManager {
	return &ReconcilerManager{
		reconcilers: make([]Reconcilable, 0),
	}
}

func (r *ReconcilerManager) AddReconciler(reconciler Reconcilable) {
	r.reconcilers = append(r.reconcilers, reconciler)
}

// ReconcileAll reconciles all registered reconcilers in the order they were added.
// The reconciliation process stops at the first reconciler that returns done=false,
// ensuring dependencies between reconcilers are respected.
func (r *ReconcilerManager) ReconcileAll(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	for _, reconciler := range r.reconcilers {
		result, done, err := reconciler.Reconcile(ctx)

		// Stop reconciliation chain if current reconciler is not done
		if !done {
			if err != nil {
				name := reflect.TypeOf(reconciler).String()
				return result, false, errors.Wrap(err, fmt.Sprintf("%s reconciliation failed", name))
			} else {
				return result, false, nil
			}
		}
	}

	return reconcile.Result{}, true, nil
}

// FinalizeAll invokes the Finalize method on all registered reconcilers.
// Unlike ReconcileAll, this method continues executing all finalizers even if one fails,
// ensuring all cleanup operations have a chance to run.
// Returns true if all finalizers completed successfully, false if any failed.
func (r *ReconcilerManager) FinalizeAll(ctx *chetypes.DeployContext) (doneAll bool) {
	doneAll = true

	for _, reconciler := range r.reconcilers {
		done := reconciler.Finalize(ctx)
		doneAll = doneAll && done
	}

	return doneAll
}
