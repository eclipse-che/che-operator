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

package deploy

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncServiceAccountToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := SyncServiceAccountToCluster(ctx, "test")
	assert.NoError(t, err)

	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		ctx.Context,
		types.NamespacedName{Name: "test", Namespace: ctx.CheCluster.Namespace},
		&corev1.ServiceAccount{},
	)
	assert.NoError(t, err)
	assert.True(t, exists)
}
