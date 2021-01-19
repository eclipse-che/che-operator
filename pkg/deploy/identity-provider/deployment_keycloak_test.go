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
package identity_provider

import (
	"os"

	"github.com/eclipse/che-operator/pkg/util"

	"github.com/eclipse/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"testing"
)

func TestDeploymentLimits(t *testing.T) {
	type testCase struct {
		name          string
		initObjects   []runtime.Object
		memoryLimit   string
		memoryRequest string
		cpuLimit      string
		cheCluster    *orgv1.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   deploy.DefaultIdentityProviderMemoryLimit,
			memoryRequest: deploy.DefaultIdentityProviderMemoryRequest,
			cpuLimit:      deploy.DefaultIdentityProviderCpuLimit,
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name:          "Test custom limits",
			initObjects:   []runtime.Object{},
			cpuLimit:      "100m",
			memoryLimit:   "200Mi",
			memoryRequest: "100Mi",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Auth: orgv1.CheClusterSpecAuth{
						IdentityProviderContainerResources: orgv1.ResourcesCustomSettings{
							Limits: orgv1.Resources{
								Cpu:    "100m",
								Memory: "200Mi",
							},
							Requests: orgv1.Resources{
								Memory: "100Mi",
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

			deployment, err := GetSpecKeycloakDeployment(deployContext, nil)
			if err != nil {
				t.Fatalf("Error creating deployment: %v", err)
			}

			actualQuantity := deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory()
			expectedQuantity := util.GetResourceQuantity(testCase.memoryLimit, testCase.memoryLimit)
			if !actualQuantity.Equal(expectedQuantity) {
				t.Errorf("Memory limit expected %s, actual %s", expectedQuantity.String(), actualQuantity.String())
			}

			actualQuantity = deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu()
			expectedQuantity = util.GetResourceQuantity(testCase.cpuLimit, testCase.cpuLimit)
			if !actualQuantity.Equal(expectedQuantity) {
				t.Errorf("CPU limit expected %s, actual %s", expectedQuantity.String(), actualQuantity.String())
			}

			actualQuantity = deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory()
			expectedQuantity = util.GetResourceQuantity(testCase.memoryRequest, testCase.memoryRequest)
			if !actualQuantity.Equal(expectedQuantity) {
				t.Errorf("Memory request expected %s, actual %s", expectedQuantity.String(), actualQuantity.String())
			}
		})
	}
}

func TestDeploymentSecurityContext(t *testing.T) {
	type testCase struct {
		name        string
		initObjects []runtime.Object
	}

	testCases := []testCase{
		{
			name:        "Test default limits deployment",
			initObjects: []runtime.Object{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: &orgv1.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "eclipse-che",
					},
				},
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
			}

			deployment, err := GetSpecKeycloakDeployment(deployContext, nil)
			if err != nil {
				t.Fatalf("Error creating deployment: %v", err)
			}

			if deployment.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop[0] != "ALL" {
				t.Error("Deployment doesn't contain 'Capabilities Drop ALL' in a SecurityContext")
			}
		})
	}
}
