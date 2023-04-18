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
package rbac

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileWorkspacePermissions(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	type testCase struct {
		name       string
		checluster *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name: "Test case #1",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.checluster, []runtime.Object{})

			up := NewUserPermissionsReconciler()
			_, done, err := up.Reconcile(ctx)

			assert.Nil(t, err)
			assert.True(t, done)

			name := fmt.Sprintf(CheUserPermissionsTemplateName, ctx.CheCluster.Namespace)
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))

			done = up.Finalize(ctx)
			assert.True(t, done)

			assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
			assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))
		})
	}
}
