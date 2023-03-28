//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package migration

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheClusterDefaultsCleaner(t *testing.T) {
	type testCase struct {
		name                                      string
		infra                                     infrastructure.Type
		cheCluster                                *chev2.CheCluster
		expectedOpenVSXURL                        *string
		expectedDisableContainerBuildCapabilities *bool
	}

	testCases := []testCase{
		{
			name:  "Test upgrade from next",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultEditor: "che-incubator/che-code/insiders",
						DefaultComponents: []devfile.Component{
							{
								Name: "universal-developer-image",
								ComponentUnion: devfile.ComponentUnion{
									Container: &devfile.ContainerComponent{
										Container: devfile.Container{
											Image: "quay.io/devfile/universal-developer-image:ubi8-38da5c2",
										},
									},
								},
							},
						},
						DisableContainerBuildCapabilities: pointer.BoolPtr(false),
					},
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.StringPtr("https://open-vsx.org"),
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
		},
		{
			name:  "Test upgrade from v7.52.0",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.52.0",
				},
			},
			expectedOpenVSXURL: pointer.StringPtr("https://open-vsx.org"),
		},
		{
			name:  "Test upgrade from v7.62.0",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.StringPtr("https://open-vsx.org"),
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.62.0",
				},
			},
		},
		{
			name:  "Test installing a new version",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.StringPtr("https://open-vsx.org"),
						},
					},
				},
			},
		},
		{
			name:  "Test use embedded OpenVSXURL after upgrade",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.62.0",
				},
			},
			expectedOpenVSXURL: pointer.StringPtr(""),
		},
		{
			name:  "Keep existed OpenVSXURL after upgrade",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.StringPtr("https://bla-bla-bla"),
						},
					},
				},
			},
			expectedOpenVSXURL: pointer.StringPtr("https://bla-bla-bla"),
		},
		{
			name:  "Keep empty OpenVSXURL after upgrade",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.StringPtr(""),
						},
					},
				},
			},
			expectedOpenVSXURL: pointer.StringPtr(""),
		},
		{
			name:  "Disable container build capabilities on Kubernetes",
			infra: infrastructure.Kubernetes,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedDisableContainerBuildCapabilities: pointer.BoolPtr(true),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			infrastructure.InitializeForTesting(testCase.infra)

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			cheClusterDefaultsCleanup := NewCheClusterDefaultsCleaner()

			_, done, err := cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			deploy.ReloadCheClusterCR(ctx)

			assert.Equal(t, testCase.expectedOpenVSXURL, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
			assert.Equal(t, testCase.expectedDisableContainerBuildCapabilities, ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities)
			assert.Empty(t, ctx.CheCluster.Spec.DevEnvironments.DefaultEditor)
			assert.Empty(t, ctx.CheCluster.Spec.DevEnvironments.DefaultComponents)
			assert.Nil(t, ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["spec.components.pluginRegistry.openVSXURL"])
			assert.Equal(t, "true", cheClusterFields["spec.components.dashboard.headerMessage"])
			assert.Equal(t, "true", cheClusterFields["spec.devEnvironments.disableContainerBuildCapabilities"])
			assert.Equal(t, "true", cheClusterFields["spec.devEnvironments.defaultComponents"])
			assert.Equal(t, "true", cheClusterFields["spec.devEnvironments.defaultEditor"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedOpenVSXURL, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
			assert.Equal(t, testCase.expectedDisableContainerBuildCapabilities, ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities)
			assert.Empty(t, ctx.CheCluster.Spec.DevEnvironments.DefaultEditor)
			assert.Empty(t, ctx.CheCluster.Spec.DevEnvironments.DefaultComponents)
			assert.Nil(t, ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage)
		})
	}
}
