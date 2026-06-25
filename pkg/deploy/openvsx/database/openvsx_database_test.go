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

package database

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestReconcileCreatesResources(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
}

func TestReconcileDeletesResourcesWhenDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))

	ctx.CheCluster.Spec.Components.OpenVSXRegistry.Enabled = ptr.To(false)
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
}

func TestFinalize(t *testing.T) {
	reconciler := NewOpenVSXDatabaseReconciler()
	assert.True(t, reconciler.Finalize(nil))
}
