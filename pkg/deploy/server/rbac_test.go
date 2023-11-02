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

package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncPermissions(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	type testCase struct {
		name       string
		checluster *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name: "Test case #1",
			checluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ClusterRoles: []string{"test-role"},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.checluster, []runtime.Object{})

			reconciler := NewCheServerReconciler()

			done, err := reconciler.syncPermissions(ctx)
			assert.True(t, done)
			assert.Nil(t, err)

			names := []string{
				fmt.Sprintf(commonPermissionsTemplateName, ctx.CheCluster.Namespace),
				fmt.Sprintf(namespacePermissionsTemplateName, ctx.CheCluster.Namespace),
				fmt.Sprintf(devWorkspacePermissionsTemplateName, ctx.CheCluster.Namespace),
			}

			for _, name := range names {
				assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
				assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))
			}
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test-role"}, &rbac.ClusterRoleBinding{}))

			done = reconciler.deletePermissions(ctx)
			assert.True(t, done)

			for _, name := range names {
				assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
				assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))
			}
			assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test-role"}, &rbac.ClusterRoleBinding{}))
		})
	}
}

// TestSyncClusterRoleBinding tests that CRB is deleted when no roles are specified in CR.
func TestSyncPermissionsWhenCheClusterUpdated(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					ClusterRoles: []string{"test-role"},
				},
			},
		},
	}

	ctx := test.GetDeployContext(cheCluster, []runtime.Object{})
	reconciler := NewCheServerReconciler()

	done, err := reconciler.syncPermissions(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	err = deploy.ReloadCheClusterCR(ctx)
	assert.NoError(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test-role"}, &rbac.ClusterRoleBinding{}))
	assert.Equal(t, ctx.CheCluster.Finalizers, []string{"test-role.crb.finalizers.che.eclipse.org"})

	ctx.CheCluster.Spec.Components.CheServer.ClusterRoles = []string{}
	err = ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	done, err = reconciler.syncPermissions(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Namespace: "eclipse-che", Name: "test-role"}, &rbac.ClusterRoleBinding{}))
	assert.Empty(t, ctx.CheCluster.Finalizers)
}
