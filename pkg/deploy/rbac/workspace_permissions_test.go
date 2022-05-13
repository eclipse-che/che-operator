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
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileWorkspacePermissions(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	type testCase struct {
		name        string
		initObjects []runtime.Object
		checluster  *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name:        "che-operator should delegate permission for workspaces in differ namespace than Che. WorkspaceNamespaceDefault = 'some-test-namespace'",
			initObjects: []runtime.Object{},
			checluster: &chev2.CheCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Workspaces: chev2.CheClusterSpecWorkspaces{
						DefaultNamespace: chev2.DefaultNamespace{
							Template: "some-test-namespace",
						},
					},
				},
			},
		},
		{
			name:        "che-operator should delegate permission for workspaces in differ namespace than Che. Property CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT = 'some-test-namespace'",
			initObjects: []runtime.Object{},
			checluster: &chev2.CheCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Operands: chev2.CheClusterOperands{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT": "some-test-namespace",
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.checluster, testCase.initObjects)

			wp := NewWorkspacePermissionsReconciler()
			_, done, err := wp.Reconcile(ctx)

			assert.Nil(t, err)
			assert.True(t, done)
			assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, CheWorkspacesClusterPermissionsFinalizerName))
			assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, NamespacesEditorPermissionsFinalizerName))
			assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, DevWorkspacePermissionsFinalizerName))

			name := "eclipse-che-cheworkspaces-clusterrole"
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))

			name = "eclipse-che-cheworkspaces-namespaces-clusterrole"
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))

			name = "eclipse-che-cheworkspaces-devworkspace-clusterrole"
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
			assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))
		})
	}
}
