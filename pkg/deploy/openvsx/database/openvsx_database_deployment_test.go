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
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

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
			memoryLimit:   constants.OpenVSXDatabaseMemoryLimit,
			memoryRequest: constants.OpenVSXDatabaseMemoryRequest,
			cpuLimit:      constants.OpenVSXDatabaseCpuLimit,
			cpuRequest:    constants.OpenVSXDatabaseCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						OpenVSXRegistry: chev2.OpenVSXRegistry{
							Enabled: ptr.To(true),
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
						OpenVSXRegistry: chev2.OpenVSXRegistry{
							Enabled: ptr.To(true),
							Database: &chev2.OpenVSXDatabase{
								Deployment: &chev2.Deployment{
									Containers: []chev2.Container{
										{
											Name: constants.OpenVSXDatabaseComponentName,
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

			deployment, err := getDeploymentSpec(ctx)
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
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	deployment, err := getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]

	secretEnvMap := make(map[string]string)
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			secretEnvMap[env.Name] = env.ValueFrom.SecretKeyRef.Key
		}
	}

	assert.Equal(t, "database-user", secretEnvMap["POSTGRESQL_USER"])
	assert.Equal(t, "database-password", secretEnvMap["POSTGRESQL_PASSWORD"])
	assert.Equal(t, "database-name", secretEnvMap["POSTGRESQL_DATABASE"])
}

func TestDeploymentSpecEnvVarsWithCustomSecret(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled:               ptr.To(true),
					CredentialsSecretName: ptr.To("custom-secret"),
				},
			},
		},
	}).Build()

	deployment, err := getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]

	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			assert.Equal(t, "custom-secret", env.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
		}
	}
}

func TestDeploymentSpecProbes(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	deployment, err := getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]

	assert.NotNil(t, container.ReadinessProbe)
	assert.NotNil(t, container.ReadinessProbe.Exec)
	assert.Equal(t, int32(15), container.ReadinessProbe.InitialDelaySeconds)

	assert.NotNil(t, container.LivenessProbe)
	assert.NotNil(t, container.LivenessProbe.TCPSocket)
	assert.Equal(t, intstr.FromInt32(constants.OpenVSXDatabaseServicePort), container.LivenessProbe.TCPSocket.Port)
	assert.Equal(t, int32(30), container.LivenessProbe.InitialDelaySeconds)
}

func TestDeploymentSpecVolumes(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	deployment, err := getDeploymentSpec(ctx)
	assert.NoError(t, err)

	volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, constants.OpenVSXDatabaseComponentName)
	assert.NotNil(t, volume.PersistentVolumeClaim)
	assert.Equal(t, constants.OpenVSXDatabaseComponentName, volume.PersistentVolumeClaim.ClaimName)

	mount := test.FindVolumeMount(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, constants.OpenVSXDatabaseComponentName)
	assert.Equal(t, "/var/lib/pgsql/data", mount.MountPath)
}

func TestDeploymentSpecStrategy(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	deployment, err := getDeploymentSpec(ctx)
	assert.NoError(t, err)

	assert.Equal(t, appsv1.RecreateDeploymentStrategyType, deployment.Spec.Strategy.Type)
}
