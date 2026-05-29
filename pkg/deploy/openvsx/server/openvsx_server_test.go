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

package server

import (
	"context"
	"strings"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
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

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: configMapName, Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
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

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))

	ctx.CheCluster.Spec.Components.OpenVSX.Enable = false
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: configMapName, Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
}

func TestGetDeploymentSpec(t *testing.T) {
	memoryRequest := resource.MustParse("512Mi")
	cpuRequest := resource.MustParse("200m")
	memoryLimit := resource.MustParse("2Gi")
	cpuLimit := resource.MustParse("2")

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
			memoryLimit:   constants.DefaultOpenVSXServerMemoryLimit,
			memoryRequest: constants.DefaultOpenVSXServerMemoryRequest,
			cpuLimit:      "0",
			cpuRequest:    constants.DefaultOpenVSXServerCpuRequest,
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
			cpuLimit:      "2",
			cpuRequest:    "200m",
			memoryLimit:   "2Gi",
			memoryRequest: "512Mi",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						OpenVSX: chev2.OpenVSX{
							Enable: true,
							Server: &chev2.OpenVSXServer{
								Deployment: &chev2.Deployment{
									Containers: []chev2.Container{
										{
											Name: constants.OpenVSXServerName,
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

			reconciler := NewOpenVSXServerReconciler()
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

func TestDeploymentSpecEnvVars(t *testing.T) {
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

	reconciler := NewOpenVSXServerReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]

	secretEnvMap := make(map[string]string)
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			secretEnvMap[env.Name] = env.ValueFrom.SecretKeyRef.Key
		}
	}

	assert.Equal(t, "user", secretEnvMap["DB_USERNAME"])
	assert.Equal(t, "password", secretEnvMap["DB_PASSWORD"])
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

	reconciler := NewOpenVSXServerReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]

	assert.NotNil(t, container.ReadinessProbe)
	assert.NotNil(t, container.ReadinessProbe.HTTPGet)
	assert.Equal(t, "/actuator/health", container.ReadinessProbe.HTTPGet.Path)

	assert.NotNil(t, container.LivenessProbe)
	assert.NotNil(t, container.LivenessProbe.HTTPGet)
	assert.Equal(t, "/actuator/health", container.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(60), container.LivenessProbe.InitialDelaySeconds)
}

func TestDeploymentSpecVolumes(t *testing.T) {
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

	reconciler := NewOpenVSXServerReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, "config")
	assert.NotNil(t, volume.ConfigMap)
	assert.Equal(t, configMapName, volume.ConfigMap.LocalObjectReference.Name)

	mount := test.FindVolumeMount(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, "config")
	assert.Equal(t, "/home/openvsx/server/config", mount.MountPath)
	assert.True(t, mount.ReadOnly)
}

func TestApplicationConfig(t *testing.T) {
	config := applicationConfig

	assert.True(t, strings.Contains(config, "jdbc:postgresql://openvsx-postgres:5432/openvsx"))
	assert.True(t, strings.Contains(config, "${DB_USERNAME}"))
	assert.True(t, strings.Contains(config, "${DB_PASSWORD}"))
	assert.True(t, strings.Contains(config, "databasesearch:\n    enabled: true"))
	assert.True(t, strings.Contains(config, "elasticsearch:\n    enabled: false"))
	assert.False(t, strings.Contains(config, "context-path"))
}
