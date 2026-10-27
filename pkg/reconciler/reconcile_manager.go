//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

// Package reconciler provides the Reconcilable interface and the ReconcilerManager,
// which together form the ordered pipeline that provisions and cleans up
// the components of a CheCluster custom resource.
package reconciler

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconcilable is implemented by every CheCluster component reconciler.
// Each implementation owns the resources of a single component and is
// responsible for both provisioning them (Reconcile) and cleaning them up (Finalize).
type Reconcilable interface {
	// Reconcile ensures the actual state of the component matches the desired state
	// defined by the CheCluster CR. It is called repeatedly until it reports done,
	// so it must tolerate being re-invoked on partially reconciled state.
	// Returns:
	// - result: reconcile.Result to propagate to the controller, e.g. RequeueAfter
	//   when the component is not ready yet and must be retried later
	// - done: true when the component is fully reconciled; false when it is not ready
	//   or failed, which stops the pipeline until it is retried
	// - err: any error encountered during reconciliation
	Reconcile(cheCtx *chetypes.CheContext) (result reconcile.Result, done bool, err error)

	// Finalize removes the resources created by the component before the CheCluster CR is deleted.
	// It is re-invoked on every reconcile until it succeeds, so it must be idempotent.
	// Returns any error encountered during finalization.
	Finalize(cheCtx *chetypes.CheContext) (err error)
}

// ReconcilerManager executes a collection of Reconcilables as an ordered pipeline.
// Reconcilers run in registration order, and each one may rely on the components
// provisioned by the reconcilers registered before it.
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

// ReconcileAll reconciles all registered reconcilers in registration order.
// The pipeline stops at the first reconciler that returns done=false, so that
// reconcilers depending on earlier components are not run before those are ready.
func (r *ReconcilerManager) ReconcileAll(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	for _, reconciler := range r.reconcilers {
		result, done, err := reconciler.Reconcile(cheCtx)

		// Stop reconciliation chain if current reconciler is not done
		if !done {
			if err != nil {
				return result, false, fmt.Errorf("%s reconciliation failed: %w", reconcilerName(reconciler), err)
			}

			return result, false, nil
		}
	}

	return reconcile.Result{}, true, nil
}

// FinalizeAll invokes the Finalize method on all registered reconcilers.
// Unlike ReconcileAll, it continues executing all finalizers even if one fails,
// ensuring every cleanup operation has a chance to run.
// It returns an error combining the errors of all failed finalizers, each wrapped
// with the reconciler type name; failed finalizers are retried on the next
// reconciliation attempt, which is safe because Finalize is idempotent.
func (r *ReconcilerManager) FinalizeAll(cheCtx *chetypes.CheContext) error {
	var errs []error

	for _, reconciler := range r.reconcilers {
		if err := reconciler.Finalize(cheCtx); err != nil {
			errs = append(errs, fmt.Errorf("%s finalization failed: %w", reconcilerName(reconciler), err))
		}
	}

	return errors.Join(errs...)
}

func reconcilerName(reconciler Reconcilable) string {
	return strings.Trim(reflect.TypeOf(reconciler).String(), "*")
}
