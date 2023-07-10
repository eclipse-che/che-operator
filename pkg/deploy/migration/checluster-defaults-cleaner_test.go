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
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheClusterDefaultsCleanerShouldNotChangeValuesOnInstallation(t *testing.T) {
	type testCase struct {
		name       string
		infra      infrastructure.Type
		cheCluster *chev2.CheCluster
	}

	zeroResource := resource.MustParse("0")

	testCases := []testCase{
		{
			name:  "Che is being installed, nothing changed",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							Deployment: &chev2.Deployment{
								Containers: []chev2.Container{
									{
										Resources: &chev2.ResourceRequirements{
											Requests: &chev2.ResourceList{
												Memory: &zeroResource,
												Cpu:    &zeroResource,
											},
											Limits: &chev2.ResourceList{
												Memory: &zeroResource,
												Cpu:    &zeroResource,
											},
										},
									},
								},
							},
						},
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.String("https://open-vsx.org"),
						},
						Dashboard: chev2.Dashboard{
							HeaderMessage: &chev2.DashboardHeaderMessage{
								Text: "Some message",
								Show: true,
							},
						},
					},
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(false),
						DefaultEditor:                     "che-incubator/che-code/insiders",
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
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			infrastructure.InitializeForTesting(testCase.infra)

			cheClusterCopy := testCase.cheCluster.DeepCopy()

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			cheClusterDefaultsCleanup := NewCheClusterDefaultsCleaner()

			_, done, err := cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, cheClusterCopy.Spec.DevEnvironments.DefaultEditor, ctx.CheCluster.Spec.DevEnvironments.DefaultEditor)
			assert.Equal(t, cheClusterCopy.Spec.DevEnvironments.DefaultComponents, ctx.CheCluster.Spec.DevEnvironments.DefaultComponents)
			assert.Equal(t, cheClusterCopy.Spec.DevEnvironments.DisableContainerBuildCapabilities, ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities)
			assert.Equal(t, cheClusterCopy.Spec.Components.PluginRegistry.OpenVSXURL, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
			assert.Equal(t, cheClusterCopy.Spec.Components.Dashboard.HeaderMessage, ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage)
			assert.Equal(t, cheClusterCopy.Spec.Components.CheServer.Deployment.Containers[0].Resources, ctx.CheCluster.Spec.Components.CheServer.Deployment.Containers[0].Resources)
		})
	}
}

func TestCheClusterDefaultsCleanerDefaultEditor(t *testing.T) {
	type testCase struct {
		name                  string
		infra                 infrastructure.Type
		cheCluster            *chev2.CheCluster
		expectedDefaultEditor string
	}

	testCases := []testCase{
		{
			name:  "Case #1",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDefaultEditor: "",
		},
		{
			name:  "Case #2",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultEditor: "che-incubator/che-code/insiders",
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDefaultEditor: "",
		},
		{
			name:  "Case #3",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultEditor: "my/editor",
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDefaultEditor: "my/editor",
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

			assert.Equal(t, testCase.expectedDefaultEditor, ctx.CheCluster.Spec.DevEnvironments.DefaultEditor)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["spec.devEnvironments.defaultEditor"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedDefaultEditor, ctx.CheCluster.Spec.DevEnvironments.DefaultEditor)
		})
	}
}

func TestCheClusterDefaultsCleanerDefaultComponents(t *testing.T) {
	type testCase struct {
		name                      string
		infra                     infrastructure.Type
		cheCluster                *chev2.CheCluster
		expectedDefaultComponents []devfile.Component
	}

	testCases := []testCase{
		{
			name:  "Case #1",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDefaultComponents: nil,
		},
		{
			name:  "Case #2",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
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
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDefaultComponents: nil,
		},
		{
			name:  "Case #3",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultComponents: []devfile.Component{
							{
								Name: "universal-developer-image",
								ComponentUnion: devfile.ComponentUnion{
									Container: &devfile.ContainerComponent{
										Container: devfile.Container{
											Image: "my/image",
										},
									},
								},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDefaultComponents: []devfile.Component{
				{
					Name: "universal-developer-image",
					ComponentUnion: devfile.ComponentUnion{
						Container: &devfile.ContainerComponent{
							Container: devfile.Container{
								Image: "my/image",
							},
						},
					},
				},
			},
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

			assert.Equal(t, testCase.expectedDefaultComponents, ctx.CheCluster.Spec.DevEnvironments.DefaultComponents)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["spec.devEnvironments.defaultComponents"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedDefaultComponents, ctx.CheCluster.Spec.DevEnvironments.DefaultComponents)
		})
	}
}

func TestCheClusterDefaultsCleanerOpenVSXURL(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *chev2.CheCluster
		expectedOpenVSXURL *string
	}

	testCases := []testCase{
		{
			name: "Case #1",
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
					CheVersion: "next",
				},
			},
		},
		{
			name: "Case #2",
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
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedOpenVSXURL: pointer.StringPtr("https://bla-bla-bla"),
		},
		{
			name: "Case #3",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: pointer.String(""),
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			// Make it works in downstream as well
			expectedOpenVSXURL: map[bool]*string{true: nil, false: pointer.String("")}[defaults.GetPluginRegistryOpenVSXURL() == ""],
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			cheClusterDefaultsCleanup := NewCheClusterDefaultsCleaner()

			_, done, err := cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedOpenVSXURL, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["spec.components.pluginRegistry.openVSXURL"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedOpenVSXURL, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
		})
	}
}

func TestCheClusterDefaultsCleanerDashboardHeaderMessage(t *testing.T) {
	prevDefaultHeaderMessageText := os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT")
	defer func() {
		_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT", prevDefaultHeaderMessageText)
	}()

	err := os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT", ".*$%^*bla^({}'\"|?<>")
	assert.NoError(t, err)

	// re initialize defaults with new env var
	defaults.Initialize()

	type testCase struct {
		name                  string
		infra                 infrastructure.Type
		cheCluster            *chev2.CheCluster
		expectedHeaderMessage *chev2.DashboardHeaderMessage
	}

	testCases := []testCase{
		{
			name:  "Case #1",
			infra: infrastructure.Kubernetes,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedHeaderMessage: nil,
		},
		{
			name:  "Case #2",
			infra: infrastructure.Kubernetes,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						Dashboard: chev2.Dashboard{
							HeaderMessage: &chev2.DashboardHeaderMessage{
								Text: "Some message",
								Show: true,
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedHeaderMessage: &chev2.DashboardHeaderMessage{
				Text: "Some message",
				Show: true,
			},
		},
		{
			name:  "Case #3",
			infra: infrastructure.Kubernetes,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						Dashboard: chev2.Dashboard{
							HeaderMessage: &chev2.DashboardHeaderMessage{
								Text: ".*$%^*bla^({}'\"|?<>",
								Show: true,
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedHeaderMessage: nil,
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

			assert.Equal(t, testCase.expectedHeaderMessage, ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["spec.components.dashboard.headerMessage"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedHeaderMessage, ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage)
		})
	}
}

func TestCheClusterDefaultsCleanerDisableContainerBuildCapabilities(t *testing.T) {
	type testCase struct {
		name                                      string
		infra                                     infrastructure.Type
		cheCluster                                *chev2.CheCluster
		expectedDisableContainerBuildCapabilities *bool
	}

	testCases := []testCase{
		{
			name:  "OpenShift case #1",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDisableContainerBuildCapabilities: nil,
		},
		{
			name:  "OpenShift case #2",
			infra: infrastructure.OpenShiftv4,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.BoolPtr(true),
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
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

			assert.Equal(t, testCase.expectedDisableContainerBuildCapabilities, ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["spec.devEnvironments.disableContainerBuildCapabilities"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedDisableContainerBuildCapabilities, ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities)
		})
	}
}

func TestCheClusterDefaultsCleanerContainerResources(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *chev2.CheCluster
		expectedDeployment *chev2.Deployment
	}

	zeroResource := resource.MustParse("0")
	memoryLimit := resource.MustParse("512Mi")
	cpuRequest := resource.MustParse("100m")

	testCases := []testCase{
		{
			name: "Case #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDeployment: nil,
		},
		{
			name: "Case #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							Deployment: &chev2.Deployment{
								Containers: []chev2.Container{
									{
										Resources: &chev2.ResourceRequirements{
											Requests: &chev2.ResourceList{
												Cpu: &cpuRequest,
											},
											Limits: &chev2.ResourceList{
												Memory: &memoryLimit,
												Cpu:    &zeroResource,
											},
										},
									},
								},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDeployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Resources: &chev2.ResourceRequirements{
							Requests: &chev2.ResourceList{
								Cpu: &cpuRequest,
							},
							Limits: &chev2.ResourceList{
								Memory: &memoryLimit,
							},
						},
					},
				},
			},
		},
		{
			name: "Case #3",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							Deployment: &chev2.Deployment{
								Containers: []chev2.Container{
									{
										Resources: &chev2.ResourceRequirements{
											Requests: &chev2.ResourceList{
												Memory: &zeroResource,
												Cpu:    &zeroResource,
											},
											Limits: &chev2.ResourceList{
												Memory: &zeroResource,
												Cpu:    &zeroResource,
											},
										},
									},
								},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedDeployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Resources: &chev2.ResourceRequirements{
							Requests: &chev2.ResourceList{},
							Limits:   &chev2.ResourceList{},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			cheClusterDefaultsCleanup := NewCheClusterDefaultsCleaner()

			_, done, err := cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedDeployment, ctx.CheCluster.Spec.Components.CheServer.Deployment)

			cheClusterFields := cheClusterDefaultsCleanup.readCheClusterDefaultsCleanupAnnotation(ctx)
			assert.Equal(t, "true", cheClusterFields["containers.resources"])

			// run twice to check that fields are not changed
			_, done, err = cheClusterDefaultsCleanup.Reconcile(ctx)
			assert.NoError(t, err)
			assert.True(t, done)

			assert.Equal(t, testCase.expectedDeployment, ctx.CheCluster.Spec.Components.CheServer.Deployment)
		})
	}
}
