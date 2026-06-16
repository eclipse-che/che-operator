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
			memoryLimit:   constants.DefaultOpenVSXDatabaseMemoryLimit,
			memoryRequest: constants.DefaultOpenVSXDatabaseMemoryRequest,
			cpuLimit:      "0",
			cpuRequest:    constants.DefaultOpenVSXDatabaseCpuRequest,
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
							OpenVSXDatabase: &chev2.OpenVSXDatabase{
								Deployment: &chev2.Deployment{
									Containers: []chev2.Container{
										{
											Name: constants.OpenVSXDatabaseName,
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

			reconciler := NewOpenVSXDatabaseReconciler()
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
					Enable: true,
				},
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
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
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: pvcName, Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseCredentialsSecret, Namespace: "eclipse-che"}, &corev1.Secret{}))
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

	reconciler := NewOpenVSXDatabaseReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))

	ctx.CheCluster.Spec.Components.OpenVSX.Enable = false
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: pvcName, Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXDatabaseCredentialsSecret, Namespace: "eclipse-che"}, &corev1.Secret{}))
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
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseCredentialsSecret, Namespace: "eclipse-che"}, secret)
	assert.NoError(t, err)
	password := string(secret.Data["db-password"])
	assert.NotEmpty(t, password)
	assert.Equal(t, "eclipse-che", string(secret.Data["publisher-name"]))
	assert.Len(t, string(secret.Data["publisher-token"]), 32)
	assert.Equal(t, "openvsx-admin", string(secret.Data["admin-name"]))
	assert.Len(t, string(secret.Data["admin-token"]), 32)

	// syncSecret should preserve existing secret (not regenerate password)
	done, err := reconciler.syncSecret(ctx)
	assert.NoError(t, err)
	assert.True(t, done)

	secret2 := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseCredentialsSecret, Namespace: "eclipse-che"}, secret2)
	assert.NoError(t, err)
	assert.Equal(t, password, string(secret2.Data["db-password"]))
}

func TestUserProvidedSecret(t *testing.T) {
	userSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-openvsx-creds",
			Namespace: "eclipse-che",
		},
		Data: map[string][]byte{
			"db-user":         []byte("myuser"),
			"db-password":     []byte("mypassword"),
			"db-name":         []byte("mydb"),
			"publisher-name":  []byte("mypub"),
			"publisher-token": []byte("mypubtoken"),
			"admin-name":      []byte("myadmin"),
			"admin-token":     []byte("myadmintoken"),
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
					OpenVSXDatabase: &chev2.OpenVSXDatabase{
						OpenVSXSecret: "my-openvsx-creds",
					},
				},
			},
		},
	}).WithObjects(userSecret).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	_, done, err := reconciler.Reconcile(ctx)
	assert.NoError(t, err)
	assert.True(t, done)

	assert.Equal(t, "my-openvsx-creds", GetCredentialsSecretName(ctx))
}

func TestUserProvidedSecretMissing(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
					OpenVSXDatabase: &chev2.OpenVSXDatabase{
						OpenVSXSecret: "nonexistent-secret",
					},
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	_, done, err := reconciler.Reconcile(ctx)
	assert.Error(t, err)
	assert.False(t, done)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserProvidedSecretMissingKeys(t *testing.T) {
	userSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "incomplete-secret",
			Namespace: "eclipse-che",
		},
		Data: map[string][]byte{
			"db-user":     []byte("myuser"),
			"db-password": []byte("mypassword"),
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
					OpenVSXDatabase: &chev2.OpenVSXDatabase{
						OpenVSXSecret: "incomplete-secret",
					},
				},
			},
		},
	}).WithObjects(userSecret).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	_, done, err := reconciler.Reconcile(ctx)
	assert.Error(t, err)
	assert.False(t, done)
	assert.Contains(t, err.Error(), "missing required keys")
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
					Enable: true,
					OpenVSXDatabase: &chev2.OpenVSXDatabase{
						Storage: &chev2.PVC{ClaimSize: "5Gi"},
					},
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
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
					Enable: true,
				},
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	deployment, err := reconciler.getDeploymentSpec(ctx)
	assert.NoError(t, err)

	container := deployment.Spec.Template.Spec.Containers[0]
	envNames := make(map[string]string)
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			envNames[env.Name] = env.ValueFrom.SecretKeyRef.Key
		}
	}

	assert.Equal(t, "db-user", envNames["POSTGRESQL_USER"])
	assert.Equal(t, "db-password", envNames["POSTGRESQL_PASSWORD"])
	assert.Equal(t, "db-name", envNames["POSTGRESQL_DATABASE"])
}
