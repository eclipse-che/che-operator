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

package test

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func AssertReconciled(
	t *testing.T,
	cheCtx *chetypes.CheContext,
	reconcileFunc func(cheCtx *chetypes.CheContext) (result reconcile.Result, done bool, err error)) {

	result, done, err := reconcileFunc(cheCtx)
	assert.NoError(t, err)
	if done {
		assert.Equal(t, reconcile.Result{}, result)
		return
	}

	result, done, err = reconcileFunc(cheCtx)
	assert.True(t, done)
	assert.Equal(t, reconcile.Result{}, result)
	assert.NoError(t, err)
}
