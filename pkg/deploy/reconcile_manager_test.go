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
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TestReconcilable struct {
	shouldFailReconcileOnce bool
	alreadyFailed           bool
}

func NewTestReconcilable(shouldFailReconcileOnce bool) *TestReconcilable {
	return &TestReconcilable{shouldFailReconcileOnce, false}
}

func (tr *TestReconcilable) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// Fails on first invocation passes on others
	if !tr.alreadyFailed && tr.shouldFailReconcileOnce {
		tr.alreadyFailed = true
		return reconcile.Result{}, false, fmt.Errorf("Reconcile error")
	} else {
		return reconcile.Result{}, true, nil
	}
}

func (tr *TestReconcilable) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func TestShouldUpdateAndCleanStatus(t *testing.T) {
	deployContext := test.GetDeployContext(nil, []runtime.Object{})

	tr := NewTestReconcilable(true)

	rm := NewReconcileManager()
	rm.RegisterReconciler(tr)

	_, done, err := rm.ReconcileAll(deployContext)

	assert.False(t, done)
	assert.NotNil(t, err)
	assert.NotEmpty(t, deployContext.CheCluster.Status.Reason)
	assert.Equal(t, "Reconciler failed deploy.TestReconcilable, cause: Reconcile error", deployContext.CheCluster.Status.Message)
	assert.Equal(t, tr, rm.failedReconciler)

	_, done, err = rm.ReconcileAll(deployContext)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Empty(t, deployContext.CheCluster.Status.Reason)
	assert.Empty(t, deployContext.CheCluster.Status.Message)
	assert.Nil(t, rm.failedReconciler)
}
