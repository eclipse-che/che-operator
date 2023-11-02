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
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TestReconcilable struct {
	shouldFailReconcileOnce bool
	shouldFailFinalizerOnce bool
	alreadyFailedReconcile  bool
	alreadyFailedFinalizer  bool
}

func NewTestReconcilable(shouldFailReconcileOnce bool, shouldFailFinalizerOnce bool) *TestReconcilable {
	return &TestReconcilable{
		shouldFailReconcileOnce,
		shouldFailFinalizerOnce,
		false,
		false}
}

func (tr *TestReconcilable) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// Fails on first invocation passes on others
	if !tr.alreadyFailedReconcile && tr.shouldFailReconcileOnce {
		tr.alreadyFailedReconcile = true
		return reconcile.Result{}, false, fmt.Errorf("reconcile error")
	} else {
		return reconcile.Result{}, true, nil
	}
}

func (tr *TestReconcilable) Finalize(ctx *chetypes.DeployContext) bool {
	// Fails on first invocation passes on others
	if !tr.alreadyFailedFinalizer && tr.shouldFailFinalizerOnce {
		tr.alreadyFailedFinalizer = true
		return false
	} else {
		return true
	}
}

func TestShouldUpdateAndCleanStatus(t *testing.T) {
	deployContext := test.GetDeployContext(nil, []runtime.Object{})

	tr := NewTestReconcilable(true, false)

	rm := NewReconcileManager()
	rm.RegisterReconciler(tr)

	_, done, err := rm.ReconcileAll(deployContext)

	assert.False(t, done)
	assert.NotNil(t, err)
	assert.NotEmpty(t, deployContext.CheCluster.Status.Reason)
	assert.Equal(t, "Reconciler failed deploy.TestReconcilable, cause: reconcile error", deployContext.CheCluster.Status.Message)
	assert.Equal(t, tr, rm.failedReconciler)

	_, done, err = rm.ReconcileAll(deployContext)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Empty(t, deployContext.CheCluster.Status.Reason)
	assert.Empty(t, deployContext.CheCluster.Status.Message)
	assert.Nil(t, rm.failedReconciler)
}

func TestShouldCleanUpAllFinalizers(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	rm := NewReconcileManager()
	rm.RegisterReconciler(NewTestReconcilable(false, false))

	_, done, err := rm.ReconcileAll(ctx)
	assert.True(t, done)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ctx.CheCluster.Finalizers))

	done = rm.FinalizeAll(ctx)
	assert.True(t, done)
	assert.Empty(t, ctx.CheCluster.Finalizers)
}

func TestShouldNotCleanUpAllFinalizersIfFailure(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	rm := NewReconcileManager()
	rm.RegisterReconciler(NewTestReconcilable(false, true))

	_, done, err := rm.ReconcileAll(ctx)
	assert.True(t, done)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ctx.CheCluster.Finalizers))

	done = rm.FinalizeAll(ctx)
	assert.False(t, done)
	assert.Equal(t, 1, len(ctx.CheCluster.Finalizers))

	done = rm.FinalizeAll(ctx)
	assert.True(t, done)
	assert.Empty(t, ctx.CheCluster.Finalizers)
}
