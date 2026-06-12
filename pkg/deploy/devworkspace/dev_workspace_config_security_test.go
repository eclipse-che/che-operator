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

package devworkspace

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileDevWorkspaceConfigForContainerCapabilities(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	var unmasked = corev1.UnmaskedProcMount
	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig without Container Security Context if all capabilities disabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(true),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with Container Security Context if container build enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(false),
						DisableContainerRunCapabilities:   ptr.To(true),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerSecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SETGID",
								"SETUID",
							},
						},
						AllowPrivilegeEscalation: ptr.To(true),
					},
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with Container Security Context if container run enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						WorkspacesPodAnnotations: map[string]string{
							"annotation_1": "value_1",
							"annotation_2": "value_1",
						},
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(false),
						ContainerRunConfiguration: &chev2.ContainerRunConfiguration{
							OpenShiftSecurityContextConstraint: "container-run",
							WorkspacesPodAnnotations: map[string]string{
								"annotation_1": "value_2",
								"annotation_3": "value_2",
							},
							ContainerSecurityContext: &corev1.SecurityContext{
								ProcMount: &unmasked,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SETUID", "SETGID"},
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					HostUsers: ptr.To(false),
					PodAnnotations: map[string]string{
						"annotation_1": "value_2",
						"annotation_2": "value_1",
						"annotation_3": "value_2",
					},
					ContainerSecurityContext: &corev1.SecurityContext{
						ProcMount: &unmasked,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"SETUID", "SETGID"},
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by adding Container Security Context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(false),
						DisableContainerRunCapabilities:   ptr.To(true),
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Workspace: &controllerv1alpha1.WorkspaceConfig{},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerSecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SETGID",
								"SETUID",
							},
						},
						AllowPrivilegeEscalation: ptr.To(true),
					},
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by removing Container Security Context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(true),
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Workspace: &controllerv1alpha1.WorkspaceConfig{
							ContainerSecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"SETGID",
									"SETUID",
								},
							},
								AllowPrivilegeEscalation: ptr.To(true),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with Container Security Context if all capabilities enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						WorkspacesPodAnnotations: map[string]string{
							"annotation_1": "value_1",
							"annotation_2": "value_1",
						},
						DisableContainerBuildCapabilities: ptr.To(false),
						DisableContainerRunCapabilities:   ptr.To(false),
						ContainerRunConfiguration: &chev2.ContainerRunConfiguration{
							OpenShiftSecurityContextConstraint: "container-run",
							WorkspacesPodAnnotations: map[string]string{
								"annotation_1": "value_2",
								"annotation_3": "value_2",
							},
							ContainerSecurityContext: &corev1.SecurityContext{
								ProcMount: &unmasked,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SETUID", "SETGID"},
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					HostUsers: ptr.To(false),
					PodAnnotations: map[string]string{
						"annotation_1": "value_2",
						"annotation_2": "value_1",
						"annotation_3": "value_2",
					},
					ContainerSecurityContext: &corev1.SecurityContext{
						ProcMount: &unmasked,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"SETUID", "SETGID"},
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)

			diff := cmp.Diff(testCase.expectedOperatorConfig, dwoc.Config,
				cmp.Options{
					cmpopts.IgnoreFields(controllerv1alpha1.WorkspaceConfig{}, "ServiceAccount", "ProjectCloneConfig", "DeploymentStrategy", "DefaultStorageSize", "StorageClassName"),
					cmpopts.IgnoreFields(controllerv1alpha1.RoutingConfig{}, "TLSCertificateConfigmapRef"),
				})
			assert.Empty(t, diff)
		})
	}
}

func TestReconcileDevWorkspaceContainerSecurityContext(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with Container security context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						// We disable container build capabilities so that it does not override the container security context we configured
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(true),
						Security: chev2.WorkspaceSecurityConfig{
							ContainerSecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"SYS_TIME",
										"SETGID",
										"SETUID",
									},
									Drop: []corev1.Capability{
										"CHOWN",
										"KILL",
									},
								},
								AllowPrivilegeEscalation: ptr.To(false),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerSecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SYS_TIME",
								"SETGID",
								"SETUID",
							},
							Drop: []corev1.Capability{
								"CHOWN",
								"KILL",
							},
						},
						AllowPrivilegeEscalation: ptr.To(false),
					},
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig when Container security context is added",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(true),
						Security: chev2.WorkspaceSecurityConfig{
							ContainerSecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"SYS_TIME",
										"SETGID",
										"SETUID",
									},
									Drop: []corev1.Capability{
										"CHOWN",
										"KILL",
									},
								},
								AllowPrivilegeEscalation: ptr.To(false),
							}},
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerSecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SYS_TIME",
								"SETGID",
								"SETUID",
							},
							Drop: []corev1.Capability{
								"CHOWN",
								"KILL",
							},
						},
						AllowPrivilegeEscalation: ptr.To(false),
					},
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig when Container security is changed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(true),
						Security: chev2.WorkspaceSecurityConfig{
							ContainerSecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"SYS_TIME",
										"SETGID",
										"SETUID",
									},
									Drop: []corev1.Capability{
										"CHOWN",
										"KILL",
									},
								},
								AllowPrivilegeEscalation: ptr.To(false),
							},
						},
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Workspace: &controllerv1alpha1.WorkspaceConfig{
							ContainerSecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"KILL",
									},
									Drop: []corev1.Capability{
										"SYS_TIME",
									},
								},
								AllowPrivilegeEscalation: ptr.To(true),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerSecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SYS_TIME",
								"SETGID",
								"SETUID",
							},
							Drop: []corev1.Capability{
								"CHOWN",
								"KILL",
							},
						},
						AllowPrivilegeEscalation: ptr.To(false),
					},
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig when Container security is removed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DisableContainerRunCapabilities:   ptr.To(true),
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Workspace: &controllerv1alpha1.WorkspaceConfig{
							ContainerSecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"KILL",
									},
									Drop: []corev1.Capability{
										"SYS_TIME",
									},
								},
								AllowPrivilegeEscalation: ptr.To(true),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)

			sortCapabilities := func(capabilities []corev1.Capability) func(i, j int) bool {
				return func(i, j int) bool {
					return capabilities[i] > capabilities[j]
				}
			}
			expectedContainerSecurityContext := testCase.expectedOperatorConfig.Workspace.ContainerSecurityContext
			actualContainerSecurityContext := dwoc.Config.Workspace.ContainerSecurityContext
			if expectedContainerSecurityContext != nil {
				sort.Slice(expectedContainerSecurityContext.Capabilities.Add, sortCapabilities(expectedContainerSecurityContext.Capabilities.Add))
				sort.Slice(expectedContainerSecurityContext.Capabilities.Drop, sortCapabilities(expectedContainerSecurityContext.Capabilities.Drop))
			}
			if actualContainerSecurityContext != nil {
				sort.Slice(actualContainerSecurityContext.Capabilities.Add, sortCapabilities(actualContainerSecurityContext.Capabilities.Add))
				sort.Slice(actualContainerSecurityContext.Capabilities.Drop, sortCapabilities(actualContainerSecurityContext.Capabilities.Drop))

			}

			assert.Equal(t, expectedContainerSecurityContext, actualContainerSecurityContext,
				fmt.Sprintf("Did not get expected ContainerSecurityContext.\nDiff:%s", cmp.Diff(expectedContainerSecurityContext, actualContainerSecurityContext)))
		})
	}
}

func TestReconcileDevWorkspacePodSecurityContext(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	configuredPodSecurityContext := &corev1.PodSecurityContext{
		RunAsUser:    ptr.To(int64(0)),
		RunAsGroup:   ptr.To(int64(0)),
		RunAsNonRoot: ptr.To(false),
	}

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with Pod security context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						Security: chev2.WorkspaceSecurityConfig{
							PodSecurityContext: configuredPodSecurityContext,
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PodSecurityContext: configuredPodSecurityContext,
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig with Pod security context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						Security: chev2.WorkspaceSecurityConfig{
							PodSecurityContext: configuredPodSecurityContext,
						},
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PodSecurityContext: configuredPodSecurityContext,
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig when Pod security context is changed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						Security: chev2.WorkspaceSecurityConfig{
							PodSecurityContext: configuredPodSecurityContext,
						},
					},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Workspace: &controllerv1alpha1.WorkspaceConfig{
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser:    ptr.To(int64(1000)),
								RunAsGroup:   ptr.To(int64(10001)),
								RunAsNonRoot: ptr.To(true),
								SupplementalGroups: []int64{
									5,
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PodSecurityContext: configuredPodSecurityContext,
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig when Pod security context is removed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{},
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Workspace: &controllerv1alpha1.WorkspaceConfig{
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser:    ptr.To(int64(1000)),
								RunAsGroup:   ptr.To(int64(10001)),
								RunAsNonRoot: ptr.To(true),
								SupplementalGroups: []int64{
									5,
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.PodSecurityContext, dwoc.Config.Workspace.PodSecurityContext,
				fmt.Sprintf("Did not get expected PodSecurityContext.\nDiff:%s", cmp.Diff(testCase.expectedOperatorConfig.Workspace.PodSecurityContext, dwoc.Config.Workspace.PodSecurityContext)))
		})
	}
}
