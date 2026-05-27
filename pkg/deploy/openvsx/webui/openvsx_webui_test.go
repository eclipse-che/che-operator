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

package webui

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileCreatesResources(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXWebUIReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: gateway.GatewayConfigMapNamePrefix + constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
}

func TestReconcileDeletesResourcesWhenDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXWebUIReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))

	ctx.CheCluster.Spec.Components.OpenVSX.Enable = false
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: gateway.GatewayConfigMapNamePrefix + constants.OpenVSXWebUIName, Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
}

func TestGatewayConfig(t *testing.T) {
	reconciler := NewOpenVSXWebUIReconciler()
	cfg := reconciler.createGatewayConfig()

	assert.Contains(t, cfg.HTTP.Routers[constants.OpenVSXWebUIName].Rule, "PathPrefix(`/openvsx`)")
	assert.Equal(t, 10, cfg.HTTP.Routers[constants.OpenVSXWebUIName].Priority)
	assert.Equal(t, "http://"+constants.OpenVSXWebUIName+":3000", cfg.HTTP.Services[constants.OpenVSXWebUIName].LoadBalancer.Servers[0].URL)

	assetsRouter := cfg.HTTP.Routers[constants.OpenVSXWebUIName+"-assets"]
	assert.NotNil(t, assetsRouter)
	assert.Equal(t, 5, assetsRouter.Priority)
	assert.Equal(t, constants.OpenVSXWebUIName, assetsRouter.Service)
}

func TestGetDeploymentSpec(t *testing.T) {
	memoryRequest := resource.MustParse("256Mi")
	cpuRequest := resource.MustParse("100m")
	memoryLimit := resource.MustParse("1Gi")
	cpuLimit := resource.MustParse("1")

	type testCase struct {
		name          string
		memoryLimit   string
		memoryRequest string
		cpuRequest    string
		cpuLimit      string
		cheCluster    *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default resource limits",
			memoryLimit:   constants.DefaultOpenVSXWebUIMemoryLimit,
			memoryRequest: constants.DefaultOpenVSXWebUIMemoryRequest,
			cpuLimit:      "0",
			cpuRequest:    constants.DefaultOpenVSXWebUICpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						OpenVSX: chev2.OpenVSX{
							Enable: true,
						},
					},
				},
			},
		},
		{
			name:          "Test custom resource limits",
			cpuLimit:      "1",
			cpuRequest:    "100m",
			memoryLimit:   "1Gi",
			memoryRequest: "256Mi",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						OpenVSX: chev2.OpenVSX{
							Enable: true,
							WebUI: &chev2.OpenVSXWebUI{
								Deployment: &chev2.Deployment{
									Containers: []chev2.Container{
										{
											Name: constants.OpenVSXWebUIName,
											Resources: &chev2.ResourceRequirements{
												Requests: &chev2.ResourceList{
													Memory: &memoryRequest,
													Cpu:    &cpuRequest,
												},
												Limits: &chev2.ResourceList{
													Memory: &memoryLimit,
													Cpu:    &cpuLimit,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).Build()

			reconciler := NewOpenVSXWebUIReconciler()
			deployment, err := reconciler.getDeploymentSpec(ctx)
			assert.NoError(t, err)

			test.CompareResources(deployment,
				test.TestExpectedResources{
					MemoryLimit:   testCase.memoryLimit,
					MemoryRequest: testCase.memoryRequest,
					CpuRequest:    testCase.cpuRequest,
					CpuLimit:      testCase.cpuLimit,
				},
				t)

			test.ValidateSecurityContext(deployment, t)
		})
	}
}

func TestDeploymentSpecProbes(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
				},
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	reconciler := NewOpenVSXWebUIReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]

	assert.NotNil(t, container.ReadinessProbe)
	assert.NotNil(t, container.ReadinessProbe.HTTPGet)
	assert.Equal(t, "/", container.ReadinessProbe.HTTPGet.Path)
	assert.Equal(t, int32(10), container.ReadinessProbe.InitialDelaySeconds)

	assert.NotNil(t, container.LivenessProbe)
	assert.NotNil(t, container.LivenessProbe.HTTPGet)
	assert.Equal(t, "/", container.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(15), container.LivenessProbe.InitialDelaySeconds)
}
