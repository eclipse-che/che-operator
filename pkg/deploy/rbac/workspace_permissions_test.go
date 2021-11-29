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

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileWorkspacePermissions(t *testing.T) {
	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})
	wp := NewWorkspacePermissionsReconciler()

	_, done, err := wp.Reconcile(ctx)

	assert.Nil(t, err)
	assert.True(t, done)
	assert.True(t, util.ContainsString(ctx.CheCluster.Finalizers, CheWorkspacesClusterPermissionsFinalizerName))
	assert.True(t, util.ContainsString(ctx.CheCluster.Finalizers, NamespacesEditorPermissionsFinalizerName))
	assert.True(t, util.ContainsString(ctx.CheCluster.Finalizers, DevWorkspacePermissionsFinalizerName))

	name := "eclipse-che-cheworkspaces-clusterrole"
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))

	name = "eclipse-che-cheworkspaces-namespaces-clusterrole"
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))

	name = "eclipse-che-cheworkspaces-devworkspace-clusterrole"
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRole{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}))
}
