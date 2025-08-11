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

package deploy

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
)

func TestSyncServiceAccountToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncServiceAccountToCluster(ctx, "test")
	assert.NoError(t, err)
	assert.True(t, done)
}
