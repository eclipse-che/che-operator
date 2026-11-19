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

package pluginregistry

import (
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/test/checontext"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"testing"
)

func TestGetPluginRegistryDeploymentSpec(t *testing.T) {
	memoryRequest := resource.MustParse("150Mi")
	cpuRequest := resource.MustParse("150m")
	memoryLimit := resource.MustParse("250Mi")
	cpuLimit := resource.MustParse("250m")

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
			name:          "Test default limits for embedded OpenVSX registry",
			initObjects:   []runtime.Object{},
			memoryLimit:   constants.DefaultPluginRegistryMemoryLimitEmbeddedOpenVSXRegistry,
			memoryRequest: constants.DefaultPluginRegistryMemoryRequestEmbeddedOpenVSXRegistry,
			cpuLimit:      "0", // CPU limit is not set when possible
			cpuRequest:    constants.DefaultPluginRegistryCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: ptr.To(""),
						},
					},
				},
			},
		},
		{
			name:          "Test default limits for external openVSX registry",
			initObjects:   []runtime.Object{},
			memoryLimit:   constants.DefaultPluginRegistryMemoryLimit,
			memoryRequest: constants.DefaultPluginRegistryMemoryRequest,
			cpuLimit:      "0", // CPU limit is not set when possible
			cpuRequest:    constants.DefaultPluginRegistryCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: ptr.To("open-vsx-url"),
						},
					},
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
						PluginRegistry: chev2.PluginRegistry{
							Deployment: &chev2.Deployment{
								Containers: []chev2.Container{
									{
										Name: constants.PluginRegistryName,
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx := checontext.NewBuilder().WithCheCluster(testCase.cheCluster).Build()

			pluginregistry := NewPluginRegistryReconciler()
			deployment, err := pluginregistry.getPluginRegistryDeploymentSpec(cheCtx)
			assert.NoError(t, err)

			resources := deployment.Spec.Template.Spec.Containers[0].Resources
			assert.True(t, resources.Limits.Memory().Equal(resource.MustParse(testCase.memoryLimit)), "memory limit")
			assert.True(t, resources.Requests.Memory().Equal(resource.MustParse(testCase.memoryRequest)), "memory request")
			assert.True(t, resources.Limits.Cpu().Equal(resource.MustParse(testCase.cpuLimit)), "CPU limit")
			assert.True(t, resources.Requests.Cpu().Equal(resource.MustParse(testCase.cpuRequest)), "CPU request")

			securityContext := deployment.Spec.Template.Spec.Containers[0].SecurityContext
			assert.Equal(t, corev1.Capability("ALL"), securityContext.Capabilities.Drop[0])
			assert.Equal(t, ptr.To(false), securityContext.AllowPrivilegeEscalation)
		})
	}
}
