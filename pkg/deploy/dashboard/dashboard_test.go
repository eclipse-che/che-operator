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
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

const Namespace = "eclipse-che"

func TestDashboardOpenShift(t *testing.T) {
	util.IsOpenShift = true

	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})
	dashboard := NewDashboardReconciler()
	_, done, err := dashboard.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &routev1.Route{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &corev1.ServiceAccount{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.False(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx), Namespace: "eclipse-che"}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx), Namespace: "eclipse-che"}, &rbacv1.ClusterRole{}))
	assert.False(t, util.ContainsString(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))
}

func TestDashboardKubernetes(t *testing.T) {
	util.IsOpenShift = false

	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})
	dashboard := NewDashboardReconciler()
	_, done, err := dashboard.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &networkingv1.Ingress{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &corev1.ServiceAccount{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.True(t, util.ContainsString(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))
}

func TestDashboardClusterRBACFinalizerOnKubernetes(t *testing.T) {
	util.IsOpenShift = false

	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})
	dashboard := NewDashboardReconciler()
	_, done, err := dashboard.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.True(t, util.ContainsString(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))

	err = dashboard.Finalize(ctx)
	assert.Nil(t, err)

	assert.False(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dashboard.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}))
	assert.False(t, util.ContainsString(ctx.CheCluster.Finalizers, ClusterPermissionsDashboardFinalizer))
}
