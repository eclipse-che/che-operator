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

package dashboard

import (
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

const Namespace = "eclipse-che"

func TestDashboardOpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	ctx := test.GetDeployContext(nil, []runtime.Object{})
	dashboard := NewDashboardReconciler()
	_, done, err := dashboard.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, &corev1.ServiceAccount{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))
}

func TestDashboardKubernetes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	ctx := test.GetDeployContext(nil, []runtime.Object{})
	dashboard := NewDashboardReconciler()
	_, done, err := dashboard.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, &corev1.ServiceAccount{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))
}

func TestDashboardClusterRBACFinalizerOnKubernetes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	ctx := test.GetDeployContext(nil, []runtime.Object{})
	dashboard := NewDashboardReconciler()
	_, done, err := dashboard.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))

	done = dashboard.Finalize(ctx)
	assert.True(t, done)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))
}
