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

package postgres

import (
	"context"
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
			memoryLimit:   constants.DefaultOpenVSXPostgresMemoryLimit,
			memoryRequest: constants.DefaultOpenVSXPostgresMemoryRequest,
			cpuLimit:      "0",
			cpuRequest:    constants.DefaultOpenVSXPostgresCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						OpenVSX: chev2.OpenVSX{
							Enabled: true,
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
							Enabled: true,
							Postgres: &chev2.OpenVSXPostgres{
								Deployment: &chev2.Deployment{
									Containers: []chev2.Container{
										{
											Name: constants.OpenVSXPostgresName,
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

			reconciler := NewOpenVSXPostgresReconciler()
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

func TestDeploymentSpecVolumes(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enabled: true,
				},
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	reconciler := NewOpenVSXPostgresReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, pvcName)
	assert.NotNil(t, volume.PersistentVolumeClaim)
	assert.Equal(t, pvcName, volume.PersistentVolumeClaim.ClaimName)

	mount := test.FindVolumeMount(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, pvcName)
	assert.Equal(t, "/var/lib/pgsql/data", mount.MountPath)
}

func TestReconcileCreatesResources(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enabled: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXPostgresReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: pvcName, Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresCredentialsSecret, Namespace: "eclipse-che"}, &corev1.Secret{}))
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
					Enabled: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXPostgresReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))

	ctx.CheCluster.Spec.Components.OpenVSX.Enabled = false
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: pvcName, Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXPostgresCredentialsSecret, Namespace: "eclipse-che"}, &corev1.Secret{}))
}

func TestReconcileSecretNotRecreated(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enabled: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXPostgresReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXPostgresCredentialsSecret, Namespace: "eclipse-che"}, secret)
	assert.NoError(t, err)
	password := string(secret.Data["password"])
	assert.NotEmpty(t, password)

	// syncSecret should preserve existing secret (not regenerate password)
	done, err := reconciler.syncSecret(ctx)
	assert.NoError(t, err)
	assert.True(t, done)

	secret2 := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXPostgresCredentialsSecret, Namespace: "eclipse-che"}, secret2)
	assert.NoError(t, err)
	assert.Equal(t, password, string(secret2.Data["password"]))
}

func TestReconcileCustomClaimSize(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enabled: true,
					Postgres: &chev2.OpenVSXPostgres{
						ClaimSize: "5Gi",
					},
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXPostgresReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	pvc := &corev1.PersistentVolumeClaim{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: "eclipse-che"}, pvc)
	assert.NoError(t, err)
	assert.Equal(t, resource.MustParse("5Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
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
					Enabled: true,
				},
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	reconciler := NewOpenVSXPostgresReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]
	envNames := make(map[string]string)
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			envNames[env.Name] = env.ValueFrom.SecretKeyRef.Key
		}
	}

	assert.Equal(t, "user", envNames["POSTGRESQL_USER"])
	assert.Equal(t, "password", envNames["POSTGRESQL_PASSWORD"])
	assert.Equal(t, "database", envNames["POSTGRESQL_DATABASE"])
}
