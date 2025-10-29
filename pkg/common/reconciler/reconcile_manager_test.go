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
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// mockReconciler is a mock implementation of Reconcilable for testing
type mockReconciler struct {
	reconcileFunc func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error)
	finalizeFunc  func(ctx *chetypes.DeployContext) bool
}

func (m *mockReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(ctx)
	}
	return reconcile.Result{}, true, nil
}

func (m *mockReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	if m.finalizeFunc != nil {
		return m.finalizeFunc(ctx)
	}
	return true
}

func TestReconcileAll_AllSucceed(t *testing.T) {
	manager := NewReconcilerManager()
	ctx := test.NewCtxBuilder().Build()

	// Add three reconcilers that all succeed
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, true, nil
		},
	})
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, true, nil
		},
	})
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, true, nil
		},
	})

	result, done, err := manager.ReconcileAll(ctx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, reconcile.Result{}, result)
}

func TestReconcileAll_FirstReconcilerFails(t *testing.T) {
	manager := NewReconcilerManager()
	ctx := test.NewCtxBuilder().Build()

	expectedErr := errors.Wrap(errors.New("test"), fmt.Sprintf("%s reconciliation failed", "reconciler.mockReconciler"))
	reconciler2Called := false
	reconciler3Called := false

	// First reconciler fails
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
			return reconcile.Result{}, false, errors.New("test")
		},
	})
	// These should not be called
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
			reconciler2Called = true
			return reconcile.Result{}, true, nil
		},
	})
	manager.AddReconciler(&mockReconciler{
		reconcileFunc: func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
			reconciler3Called = true
			return reconcile.Result{}, true, nil
		},
	})

	result, done, err := manager.ReconcileAll(ctx)

	assert.False(t, done)
	assert.Equal(t, expectedErr.Error(), err.Error())
	assert.Equal(t, reconcile.Result{}, result)
	assert.False(t, reconciler2Called)
	assert.False(t, reconciler3Called)
}

func TestFinalizeAll_AllSucceed(t *testing.T) {
	manager := NewReconcilerManager()
	ctx := test.NewCtxBuilder().Build()

	// Add three reconcilers that all finalize successfully
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(ctx *chetypes.DeployContext) bool {
			return true
		},
	})
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(ctx *chetypes.DeployContext) bool {
			return true
		},
	})
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(ctx *chetypes.DeployContext) bool {
			return true
		},
	})

	doneAll := manager.FinalizeAll(ctx)

	assert.True(t, doneAll)
}

func TestFinalizeAll_OneFails(t *testing.T) {
	manager := NewReconcilerManager()
	ctx := test.NewCtxBuilder().Build()

	reconciler1Called := false
	reconciler2Called := false
	reconciler3Called := false

	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(ctx *chetypes.DeployContext) bool {
			reconciler1Called = true
			return true
		},
	})
	// Second reconciler fails
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(ctx *chetypes.DeployContext) bool {
			reconciler2Called = true
			return false
		},
	})
	// Third should still be called even though second failed
	manager.AddReconciler(&mockReconciler{
		finalizeFunc: func(ctx *chetypes.DeployContext) bool {
			reconciler3Called = true
			return true
		},
	})

	doneAll := manager.FinalizeAll(ctx)

	assert.False(t, doneAll)
	assert.True(t, reconciler1Called)
	assert.True(t, reconciler2Called)
	assert.True(t, reconciler3Called)
}
