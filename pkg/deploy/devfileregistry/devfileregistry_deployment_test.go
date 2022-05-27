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
package devfileregistry

import (
	"os"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"testing"
)

func TestGetDevfileRegistryDeploymentSpec(t *testing.T) {
	type testCase struct {
		name          string
		initObjects   []runtime.Object
		memoryLimit   string
		memoryRequest string
		cpuRequest    string
		cpuLimit      string
		cheCluster    *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   constants.DefaultDevfileRegistryMemoryLimit,
			memoryRequest: constants.DefaultDevfileRegistryMemoryRequest,
			cpuLimit:      constants.DefaultDevfileRegistryCpuLimit,
			cpuRequest:    constants.DefaultDevfileRegistryCpuRequest,
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
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							Deployment: chev2.Deployment{
								Containers: []chev2.Container{
									{
										Name: constants.DevfileRegistryName,
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

			devfileregistry := NewDevfileRegistryReconciler()
			deployment := devfileregistry.getDevfileRegistryDeploymentSpec(ctx)

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
