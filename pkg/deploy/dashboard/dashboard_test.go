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

package dashboard

import (
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

func TestDashboardOpenShift(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := test.NewCheContextBuilder().Build()
	dashboard := NewDashboardReconciler()
	assert.NoError(t, test.ReconcileTwice(cheCtx, dashboard.Reconcile))

	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getComponentName(cheCtx), Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getComponentName(cheCtx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, &corev1.ServiceAccount{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getClusterRoleBindingName(cheCtx)}, &rbacv1.ClusterRoleBinding{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getClusterRoleName(cheCtx)}, &rbacv1.ClusterRole{}))
}

func TestDashboardKubernetes(t *testing.T) {
	test.StartOn(infrastructure.Kubernetes)

	cheCtx := test.NewCheContextBuilder().Build()
	dashboard := NewDashboardReconciler()
	assert.NoError(t, test.ReconcileTwice(cheCtx, dashboard.Reconcile))

	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getComponentName(cheCtx), Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getComponentName(cheCtx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, &corev1.ServiceAccount{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getClusterRoleBindingName(cheCtx)}, &rbacv1.ClusterRoleBinding{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getClusterRoleName(cheCtx)}, &rbacv1.ClusterRole{}))
}

func TestDashboardClusterRBACFinalizerOnKubernetes(t *testing.T) {
	test.StartOn(infrastructure.Kubernetes)

	cheCtx := test.NewCheContextBuilder().Build()
	dashboard := NewDashboardReconciler()
	assert.NoError(t, test.ReconcileTwice(cheCtx, dashboard.Reconcile))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getClusterRoleBindingName(cheCtx)}, &rbacv1.ClusterRoleBinding{}))
	assert.NoError(t, test.ExpectFound(types.NamespacedName{Name: dashboard.getClusterRoleName(cheCtx)}, &rbacv1.ClusterRole{}))

	err := dashboard.Finalize(cheCtx)
	assert.Nil(t, err)
	assert.NoError(t, test.ExpectNotFound(types.NamespacedName{Name: dashboard.getClusterRoleBindingName(cheCtx)}, &rbacv1.ClusterRoleBinding{}))
	assert.NoError(t, test.ExpectNotFound(types.NamespacedName{Name: dashboard.getClusterRoleName(cheCtx)}, &rbacv1.ClusterRole{}))
}
