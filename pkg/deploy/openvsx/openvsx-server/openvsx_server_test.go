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

package openvsx_server

import (
	"strings"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestOpenVSXServerReconciler(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	ns := "eclipse-che"
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.PersistentVolumeClaim{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.ConfigMap{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: gateway.GatewayConfigMapNamePrefix + constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.ConfigMap{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionsConfigMapName, Namespace: ns}, &corev1.ConfigMap{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionPublishJobName, Namespace: ns}, &batchv1.Job{}))

	ctx.CheCluster.Spec.Components.OpenVSXRegistry.Enable = false
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &appsv1.Deployment{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.Service{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.PersistentVolumeClaim{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.ConfigMap{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: gateway.GatewayConfigMapNamePrefix + constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.ConfigMap{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionsConfigMapName, Namespace: ns}, &corev1.ConfigMap{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionPublishJobName, Namespace: ns}, &batchv1.Job{}))
}

func TestDeploymentSpecHasWaitDatabaseInitContainer(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	if !assert.NoError(t, err) {
		return
	}

	initContainers := deployment.Spec.Template.Spec.InitContainers
	if !assert.Len(t, initContainers, 1, "expected exactly one init container") {
		return
	}

	ic := initContainers[0]
	assert.Equal(t, "wait-database", ic.Name)
	assert.Equal(t, defaults.GetOpenVSXDatabaseImage(ctx.CheCluster), ic.Image)

	expectedSecretName := openvsx.GetCredentialsSecretName(ctx)
	envMap := make(map[string]corev1.EnvVar, len(ic.Env))
	for _, e := range ic.Env {
		envMap[e.Name] = e
	}
	assert.Equal(t, constants.OpenVSXDatabaseComponentName, envMap["PGHOST"].Value)
	assert.Equal(t, expectedSecretName, envMap["PGUSER"].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, expectedSecretName, envMap["PGDATABASE"].ValueFrom.SecretKeyRef.Name)

	if assert.Len(t, ic.Command, 3) {
		assert.Contains(t, ic.Command[2], "pg_isready")
		assert.True(t, strings.Contains(ic.Command[2], "120"), "expected timeout of 120 seconds in the command")
	}

	assert.NotNil(t, ic.Resources.Requests, "init container should have resource requests")
	assert.NotNil(t, ic.Resources.Limits, "init container should have resource limits")
}
