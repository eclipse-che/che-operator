//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"os"

	"github.com/eclipse-che/che-operator/pkg/util"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

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
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
			}

			postgres := NewPostgres(deployContext)
			deployment, err := postgres.GetDeploymentSpec(nil)
			if err != nil {
				t.Fatalf("Error creating deployment: %v", err)
			}

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

func TestSyncAllToCluster(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	postgres := NewPostgres(deployContext)
	done, err := postgres.SyncAll()
	if !done || err != nil {
		t.Fatalf("Failed to sync PostgreSQL: %v", err)
	}

	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.PostgresName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: "eclipse-che"}, pvc)
	if err != nil {
		t.Fatalf("Failed to get pvc: %v", err)
	}

	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.PostgresName, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}
}
