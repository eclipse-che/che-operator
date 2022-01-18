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
package server

import (
	"context"
	"os"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

func TestReconcile(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      os.Getenv("CHE_FLAVOR"),
		},
	}

	util.IsOpenShift = true
	ctx := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	chehost := NewCheHostReconciler()
	done, err := chehost.exposeCheEndpoint(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	server := NewCheServerReconciler()
	_, done, err = server.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: getComponentName(ctx), Namespace: "eclipse-che"}, &routev1.Route{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: CheConfigMapName, Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: getComponentName(ctx), Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.NotEmpty(t, cheCluster.Status.CheURL)
	assert.NotEmpty(t, cheCluster.Status.CheClusterRunning)
	assert.NotEmpty(t, cheCluster.Status.CheVersion)
}

func TestSyncLegacyConfigMap(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
	}
	ctx := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	legacyConfigMap := deploy.GetConfigMapSpec(ctx, "custom", map[string]string{"a": "b"}, "test")
	err := ctx.ClusterAPI.Client.Create(context.TODO(), legacyConfigMap)
	assert.Nil(t, err)

	server := NewCheServerReconciler()
	done, err := server.syncLegacyConfigMap(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.False(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "custom", Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
	assert.Equal(t, cheCluster.Spec.Server.CustomCheProperties["a"], "b")
}

func TestUpdateAvailabilityStatus(t *testing.T) {
	cheDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: "eclipse-che",
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
			Replicas:          1,
		},
	}
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      os.Getenv("CHE_FLAVOR"),
		},
		Spec:   orgv1.CheClusterSpec{},
		Status: orgv1.CheClusterStatus{},
	}

	ctx := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	server := NewCheServerReconciler()
	done, err := server.updateAvailabilityStatus(ctx)
	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, cheCluster.Status.CheClusterRunning, UnavailableStatus)

	err = ctx.ClusterAPI.Client.Create(context.TODO(), cheDeployment)
	assert.Nil(t, err)

	done, err = server.updateAvailabilityStatus(ctx)
	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, cheCluster.Status.CheClusterRunning, AvailableStatus)

	cheDeployment.Status.Replicas = 2
	err = ctx.ClusterAPI.Client.Update(context.TODO(), cheDeployment)
	assert.Nil(t, err)

	done, err = server.updateAvailabilityStatus(ctx)
	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, cheCluster.Status.CheClusterRunning, RollingUpdateInProgressStatus)
}
