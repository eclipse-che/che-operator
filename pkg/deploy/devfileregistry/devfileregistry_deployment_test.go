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

	"github.com/eclipse-che/che-operator/pkg/util"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		cheCluster    *orgv1.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   deploy.DefaultDevfileRegistryMemoryLimit,
			memoryRequest: deploy.DefaultDevfileRegistryMemoryRequest,
			cpuLimit:      deploy.DefaultDevfileRegistryCpuLimit,
			cpuRequest:    deploy.DefaultDevfileRegistryCpuRequest,
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
					Server: orgv1.CheClusterSpecServer{
						DevfileRegistryCpuLimit:      "250m",
						DevfileRegistryCpuRequest:    "150m",
						DevfileRegistryMemoryLimit:   "250Mi",
						DevfileRegistryMemoryRequest: "150Mi",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
				Proxy:      &deploy.Proxy{},
				CheCluster: testCase.cheCluster,
			}

			devfileregistry := NewDevfileRegistry(deployContext)
			deployment := devfileregistry.GetDevfileRegistryDeploymentSpec()

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
