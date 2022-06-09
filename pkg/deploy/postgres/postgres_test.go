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
package postgres

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"testing"
)

func TestDeploymentSpec(t *testing.T) {
	type testCase struct {
		name          string
		initObjects   []runtime.Object
		memoryLimit   string
		memoryRequest string
		cpuLimit      string
		cpuRequest    string
		cheCluster    *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   constants.DefaultPostgresMemoryLimit,
			memoryRequest: constants.DefaultPostgresMemoryRequest,
			cpuLimit:      constants.DefaultPostgresCpuLimit,
			cpuRequest:    constants.DefaultPostgresCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
		},
		{
			name:          "Test custom limits",
			initObjects:   []runtime.Object{},
			cpuLimit:      "250m",
			cpuRequest:    "150m",
			memoryLimit:   "250Mi",
			memoryRequest: "150Mi",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						Database: chev2.Database{
							Deployment: chev2.Deployment{
								Containers: []chev2.Container{
									{
										Name: constants.PostgresName,
										Resources: chev2.ResourceRequirements{
											Requests: chev2.ResourceList{
												Memory: resource.MustParse("150Mi"),
												Cpu:    resource.MustParse("150m"),
											},
											Limits: chev2.ResourceList{
												Memory: resource.MustParse("250Mi"),
												Cpu:    resource.MustParse("250m"),
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
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			postgres := NewPostgresReconciler()

			deployment, err := postgres.getDeploymentSpec(nil, ctx)
			assert.Nil(t, err)
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

func TestPostgresReconcile(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	postgres := NewPostgresReconciler()
	_, done, err := postgres.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres", Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres-data", Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres", Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres-credentials", Namespace: "eclipse-che"}, &corev1.Secret{}))
}

func TestGetPostgresImage(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *chev2.CheCluster
		postgresDeployment *appsv1.Deployment

		expectedPostgresImage string
		expectedError         bool
	}

	testCases := []testCase{
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			expectedPostgresImage: defaults.GetPostgres13Image(&chev2.CheCluster{}),
		},
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					PostgresVersion: "13.3",
				},
			},
			expectedPostgresImage: defaults.GetPostgres13Image(&chev2.CheCluster{}),
		},
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					PostgresVersion: "13.5",
				},
			},
			expectedPostgresImage: defaults.GetPostgres13Image(&chev2.CheCluster{}),
		},
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					PostgresVersion: "9.6",
				},
			},
			expectedPostgresImage: defaults.GetPostgresImage(&chev2.CheCluster{}),
		},
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						Database: chev2.Database{
							Deployment: chev2.Deployment{
								Containers: []chev2.Container{
									chev2.Container{
										Image: "custom_postgre_image",
									},
								},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					PostgresVersion: "<some_version>",
				},
			},
			expectedPostgresImage: "custom_postgre_image",
		},
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					PostgresVersion: "<unrecognized_version>",
				},
			},
			expectedError: true,
		},

		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						Database: chev2.Database{},
					},
				},
			},
			postgresDeployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: "current_postgres_image",
								},
							},
						},
					},
				},
			},
			expectedPostgresImage: "current_postgres_image",
		},
		{
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					PostgresVersion: "13.3",
				},
			},
			postgresDeployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: "current_postgres_image",
								},
							},
						},
					},
				},
			},
			expectedPostgresImage: defaults.GetPostgres13Image(&chev2.CheCluster{}),
		},
	}

	for i, testCase := range testCases {
		actualPostgreImage, err := getPostgresImage(testCase.postgresDeployment, testCase.cheCluster)

		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			if testCase.expectedError {
				assert.NotNil(t, err, "Error expected")
			} else {
				assert.Nil(t, err, "Unexpected error occurred %v", err)
				assert.Equal(t, testCase.expectedPostgresImage, actualPostgreImage, "A wrong PostgreSQL image")
			}
		})
	}
}
