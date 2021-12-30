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

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
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
		cheCluster    *orgv1.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   deploy.DefaultPostgresMemoryLimit,
			memoryRequest: deploy.DefaultPostgresMemoryRequest,
			cpuLimit:      deploy.DefaultPostgresCpuLimit,
			cpuRequest:    deploy.DefaultPostgresCpuRequest,
			cheCluster: &orgv1.CheCluster{
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
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						ChePostgresContainerResources: orgv1.ResourcesCustomSettings{
							Limits: orgv1.Resources{
								Cpu:    "250m",
								Memory: "250Mi",
							},
							Requests: orgv1.Resources{
								Memory: "150Mi",
								Cpu:    "150m",
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

			ctx := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})
			postgres := NewPostgresReconciler()

			deployment, err := postgres.getDeploymentSpec(nil, ctx)
			assert.Nil(t, err)
			util.CompareResources(deployment,
				util.TestExpectedResources{
					MemoryLimit:   testCase.memoryLimit,
					MemoryRequest: testCase.memoryRequest,
					CpuRequest:    testCase.cpuRequest,
					CpuLimit:      testCase.cpuLimit,
				},
				t)

			util.ValidateSecurityContext(deployment, t)
		})
	}
}

func TestPostgresReconcile(t *testing.T) {
	util.IsOpenShift = true
	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})

	postgres := NewPostgresReconciler()
	_, done, err := postgres.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres", Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres-data", Namespace: "eclipse-che"}, &corev1.PersistentVolumeClaim{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "postgres", Namespace: "eclipse-che"}, &appsv1.Deployment{}))
}

func TestGetPostgresImage(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *orgv1.CheCluster
		postgresDeployment *appsv1.Deployment

		expectedPostgresImage string
		expectedError         bool
	}

	testCases := []testCase{
		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			expectedPostgresImage: deploy.DefaultPostgres13Image(&orgv1.CheCluster{}),
		},
		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						PostgresVersion: "13.3",
					},
				},
			},
			expectedPostgresImage: deploy.DefaultPostgres13Image(&orgv1.CheCluster{}),
		},
		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						PostgresVersion: "13.5",
					},
				},
			},
			expectedPostgresImage: deploy.DefaultPostgres13Image(&orgv1.CheCluster{}),
		},
		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						PostgresVersion: "9.6",
					},
				},
			},
			expectedPostgresImage: deploy.DefaultPostgresImage(&orgv1.CheCluster{}),
		},
		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						PostgresImage:   "custom_postgre_image",
						PostgresVersion: "<some_version>",
					},
				},
			},
			expectedPostgresImage: "custom_postgre_image",
		},
		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						PostgresVersion: "unrecognized_version",
					},
				},
			},
			expectedError: true,
		},

		{
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{},
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
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Database: orgv1.CheClusterSpecDB{
						PostgresVersion: "13.3",
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
			expectedPostgresImage: deploy.DefaultPostgres13Image(&orgv1.CheCluster{}),
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
