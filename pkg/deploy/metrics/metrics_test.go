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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestReconcileDWOMetrics(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				DevEnvironments: chev2.CheClusterDevEnvironments{
					Metrics: pointer.Bool(true),
				},
				Components: chev2.CheClusterComponents{
					Metrics: chev2.ServerMetrics{
						Enable: false,
					},
				},
			},
		},
	).WithObjects(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "eclipse-che",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
		},
	).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	// DWO resources should be created
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: DWOServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	ns := &corev1.Namespace{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "eclipse-che"}, ns)

	assert.NoError(t, err)
	assert.Equal(t, "true", ns.Labels[openshiftMonitoringLabel])

	// Disable metrics
	ctx.CheCluster.Spec.DevEnvironments.Metrics = pointer.Bool(false)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, !test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleName, Namespace: "openshift-operators"}, &rbacv1.Role{}))
	assert.True(t, !test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwoPrometheusRoleBindingName, Namespace: "openshift-operators"}, &rbacv1.RoleBinding{}))
	assert.True(t, !test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: DWOServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	// Label `openshift.io/cluster-monitoring` must exist
	ns = &corev1.Namespace{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "eclipse-che"}, ns)

	assert.NoError(t, err)
	assert.Equal(t, "true", ns.Labels[openshiftMonitoringLabel])
}

func TestReconcileCheServerMetricsEnabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				DevEnvironments: chev2.CheClusterDevEnvironments{
					Metrics: pointer.Bool(false),
				},
				Components: chev2.CheClusterComponents{
					Metrics: chev2.ServerMetrics{
						Enable: true,
					},
				},
			},
		},
	).WithObjects(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "eclipse-che",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
		},
	).Build()

	reconciler := NewMetricsReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	// CheServer resources should be created
	cheServerRoleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, defaults.GetCheFlavor())
	cheServerRoleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, defaults.GetCheFlavor())
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: CheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	ns := &corev1.Namespace{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "eclipse-che"}, ns)

	assert.NoError(t, err)
	assert.Equal(t, "true", ns.Labels[openshiftMonitoringLabel])

	// Disable metrics
	ctx.CheCluster.Spec.Components.Metrics.Enable = false

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)
	assert.True(t, !test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleName, Namespace: "eclipse-che"}, &rbacv1.Role{}))
	assert.True(t, !test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: cheServerRoleBindingName, Namespace: "eclipse-che"}, &rbacv1.RoleBinding{}))
	assert.True(t, !test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: CheServerServiceMonitorName, Namespace: "eclipse-che"}, &monitoringv1.ServiceMonitor{}))

	// Label `openshift.io/cluster-monitoring` must exist
	ns = &corev1.Namespace{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "eclipse-che"}, ns)

	assert.NoError(t, err)
	assert.Equal(t, "true", ns.Labels[openshiftMonitoringLabel])
}
