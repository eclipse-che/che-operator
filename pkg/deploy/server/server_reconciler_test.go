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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

func TestReconcile(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
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

	server := NewCheServerReconciler()
	_, done, err := server.Reconcile(ctx)
	assert.False(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: CheConfigMapName, Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Namespace: "eclipse-che", Name: "che"}, &corev1.ServiceAccount{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test-role"}, &rbac.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.Equal(t, ctx.CheCluster.Status.ChePhase, chev2.CheClusterPhase(chev2.ClusterPhaseInactive))
	assert.Equal(t, 1, len(ctx.CheCluster.Finalizers))

	cheDeployment := &appsv1.Deployment{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: defaults.GetCheFlavor(), Namespace: "eclipse-che"}, cheDeployment)
	assert.Nil(t, err)

	cheDeployment.Status.Replicas = 1
	cheDeployment.Status.AvailableReplicas = 1
	err = ctx.ClusterAPI.Client.Update(context.TODO(), cheDeployment)

	_, done, err = server.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.Equal(t, ctx.CheCluster.Status.ChePhase, chev2.CheClusterPhase(chev2.ClusterPhaseActive))
	assert.NotEmpty(t, ctx.CheCluster.Status.CheVersion)
	assert.NotEmpty(t, ctx.CheCluster.Status.CheURL)

	done = server.Finalize(ctx)
	assert.True(t, done)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test-role"}, &rbac.ClusterRoleBinding{}))
}

func TestUpdateAvailabilityStatus(t *testing.T) {
	cheDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor(),
			Namespace: "eclipse-che",
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
			Replicas:          1,
		},
	}
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	server := NewCheServerReconciler()
	done, err := server.syncActiveChePhase(ctx)
	assert.False(t, done)
	assert.Nil(t, err)
	assert.Equal(t, ctx.CheCluster.Status.ChePhase, chev2.CheClusterPhase(chev2.ClusterPhaseInactive))

	err = ctx.ClusterAPI.Client.Create(context.TODO(), cheDeployment)
	assert.Nil(t, err)

	done, err = server.syncActiveChePhase(ctx)
	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, ctx.CheCluster.Status.ChePhase, chev2.CheClusterPhase(chev2.ClusterPhaseActive))

	cheDeployment.Status.Replicas = 2
	err = ctx.ClusterAPI.Client.Update(context.TODO(), cheDeployment)
	assert.Nil(t, err)

	done, err = server.syncActiveChePhase(ctx)
	assert.False(t, done)
	assert.Nil(t, err)
	assert.Equal(t, ctx.CheCluster.Status.ChePhase, chev2.CheClusterPhase(chev2.RollingUpdate))
}

func TestGetFinalizerName(t *testing.T) {
	crbName := "0123456789012345678901234567890123456789" // 40 chars

	reconciler := NewCheServerReconciler()
	finalizer := reconciler.getCRBFinalizerName(crbName)

	assert.Equal(t, crbName+".crb.finalizers.che.ecl", finalizer)
	assert.True(t, len(finalizer) <= 63)
}
