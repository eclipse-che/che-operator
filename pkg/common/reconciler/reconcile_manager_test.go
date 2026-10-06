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

package reconciler

import (
	"fmt"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// mockReconciler is a mock implementation of Reconcilable for testing
type mockReconciler struct {
	reconcileFunc func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error)
	finalizeFunc  func(cheCtx *chetypes.CheContext) bool
}

func (m *mockReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(cheCtx)
	}
	return reconcile.Result{}, true, nil
}

func (m *mockReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	if m.finalizeFunc != nil {
		return m.finalizeFunc(cheCtx)
	}
	return true
}

func TestReconcileAll_AllSucceed(t *testing.T) {
	manager := NewReconcilerManager()
	cheCtx := test.NewCtxBuilder().Build()

	// Add three reconcilers that all succeed
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, true, nil
		},
	})
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, true, nil
		},
	})
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, true, nil
		},
	})

	result, done, err := manager.ReconcileAll(cheCtx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, reconcile.Result{}, result)
}

func TestReconcileAll_FirstReconcilerFails(t *testing.T) {
	manager := NewReconcilerManager()
	cheCtx := test.NewCtxBuilder().Build()

	expectedErr := errors.Wrap(errors.New("test"), fmt.Sprintf("%s reconciliation failed", "reconciler.mockReconciler"))
	reconciler2Called := false
	reconciler3Called := false

	// First reconciler fails
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, false, errors.New("test")
		},
	})
	// These should not be called
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
			reconciler2Called = true
			return reconcile.Result{}, true, nil
		},
	})
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
			reconciler3Called = true
			return reconcile.Result{}, true, nil
		},
	})

	result, done, err := manager.ReconcileAll(cheCtx)

	assert.False(t, done)
	assert.Equal(t, expectedErr.Error(), err.Error())
	assert.Equal(t, reconcile.Result{}, result)
	assert.False(t, reconciler2Called)
	assert.False(t, reconciler3Called)
}

func TestFinalizeAll_AllSucceed(t *testing.T) {
	manager := NewReconcilerManager()
	cheCtx := test.NewCtxBuilder().Build()

	// Add three reconcilers that all finalize successfully
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(cheCtx *chetypes.CheContext) bool {
			return true
		},
	})
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(cheCtx *chetypes.CheContext) bool {
			return true
		},
	})
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(cheCtx *chetypes.CheContext) bool {
			return true
		},
	})

	doneAll := manager.FinalizeAll(cheCtx)

	assert.True(t, doneAll)
}

func TestFinalizeAll_OneFails(t *testing.T) {
	manager := NewReconcilerManager()
	cheCtx := test.NewCtxBuilder().Build()

	reconciler1Called := false
	reconciler2Called := false
	reconciler3Called := false

	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(cheCtx *chetypes.CheContext) bool {
			reconciler1Called = true
			return true
		},
	})
	// Second reconciler fails
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(cheCtx *chetypes.CheContext) bool {
			reconciler2Called = true
			return false
		},
	})
	// Third should still be called even though second failed
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(cheCtx *chetypes.CheContext) bool {
			reconciler3Called = true
			return true
		},
	})

	doneAll := manager.FinalizeAll(cheCtx)

	assert.False(t, doneAll)
	assert.True(t, reconciler1Called)
	assert.True(t, reconciler2Called)
	assert.True(t, reconciler3Called)
}
