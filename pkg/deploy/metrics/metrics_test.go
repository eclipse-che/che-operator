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

package metrics

import (
	"context"
	"fmt"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

func TestReconcileMetrics(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Metrics: chev2.ServerMetrics{Enable: true},
			},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eclipse-che",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(namespace).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	cheFlavor := defaults.GetCheFlavor()

	// DWO resources
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	// Che server resources
	cheServerRoleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, cheFlavor)
	cheServerRoleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, cheFlavor)
	cheServerServiceMonitorName := cheFlavor
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	// Namespace label
	ns := &corev1.Namespace{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "eclipse-che"}, ns)
	assert.NoError(t, err)
	assert.Equal(t, "true", ns.Labels[openshiftMonitoringLabel])
}

func TestReconcileMetricsDisabled(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Metrics: chev2.ServerMetrics{Enable: false},
			},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eclipse-che",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(namespace).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	cheFlavor := defaults.GetCheFlavor()

	// DWO resources should still exist
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	// Che server resources should not exist
	cheServerRoleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, cheFlavor)
	cheServerRoleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, cheFlavor)
	cheServerServiceMonitorName := cheFlavor
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))
}

func TestServiceMonitorIntervalPreservation(t *testing.T) {
	cheFlavor := defaults.GetCheFlavor()
	cheServerServiceMonitorName := cheFlavor

	existingServiceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      cheServerServiceMonitorName,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: "30s",
					Scheme:   "http",
					Port:     metricsPortName,
				},
			},
		},
	}

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Metrics: chev2.ServerMetrics{Enable: true},
			},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eclipse-che",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(namespace, existingServiceMonitor).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	sm := &monitoringv1.ServiceMonitor{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: cheServerServiceMonitorName, Namespace: "eclipse-che"}, sm)
	assert.NoError(t, err)
	assert.Equal(t, monitoringv1.Duration("30s"), sm.Spec.Endpoints[0].Interval)
}

func TestReconcileMetricsIdempotent(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Metrics: chev2.ServerMetrics{Enable: true},
			},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eclipse-che",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(namespace).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	cheFlavor := defaults.GetCheFlavor()

	// DWO resources
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	// Che server resources
	cheServerRoleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, cheFlavor)
	cheServerRoleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, cheFlavor)
	cheServerServiceMonitorName := cheFlavor
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))
}

func TestFinalizeMetrics(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Metrics: chev2.ServerMetrics{Enable: true},
			},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eclipse-che",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(namespace).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	cheFlavor := defaults.GetCheFlavor()
	cheServerRoleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, cheFlavor)
	cheServerRoleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, cheFlavor)
	cheServerServiceMonitorName := cheFlavor

	// Verify resources exist before finalize
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	done := reconciler.Finalize(ctx)
	assert.True(t, done)

	// Verify all resources are deleted
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))
}
