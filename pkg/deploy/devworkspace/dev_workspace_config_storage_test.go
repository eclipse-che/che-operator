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
	"regexp"
	"testing"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileDevWorkspaceConfigStorage(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	type errorTestCase struct {
		name                 string
		cheCluster           *chev2.CheCluster
		existedObjects       []client.Object
		expectedErrorMessage string
	}

	var quantity15Gi = resource.MustParse("15Gi")
	var quantity10Gi = resource.MustParse("10Gi")

	var expectedErrorTestCases = []errorTestCase{
		{
			name: "Invalid claim size string",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "invalid-ClaimSize",
							},
						},
					},
				},
			},
			expectedErrorMessage: "quantities must match the regular expression",
		},
	}

	testCases := []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{Workspace: &controllerv1alpha1.WorkspaceConfig{DeploymentStrategy: "Recreate"}},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with ephemeral storage strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.EphemeralPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "10Gi",
							},
							PerWorkspaceStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "10Gi",
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{Workspace: &controllerv1alpha1.WorkspaceConfig{DeploymentStrategy: "Recreate"}},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with StorageClassName only",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName:   ptr.To("test-storage"),
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with non empty StorageAccessMode",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageAccessMode: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteMany,
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageAccessMode: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteMany,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Not setting PerUserStrategyPvcConfig should reset DevWorkspaceConfig to default StorageAccessMode",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with nil StorageAccessMode",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageAccessMode: nil,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageAccessMode:  nil,
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with non empty default container resources",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DefaultContainerResources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					DefaultContainerResources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with nil default container resources",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						DefaultContainerResources:         nil,
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					DefaultContainerResources: nil,
					DeploymentStrategy:        "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with non empty container resource caps",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						ContainerResourceCaps: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("2Gi"),
								corev1.ResourceCPU:    resource.MustParse("2000m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
								corev1.ResourceCPU:    resource.MustParse("1000m"),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerResourceCaps: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("2000m"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
							corev1.ResourceCPU:    resource.MustParse("1000m"),
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with nil container resource caps",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						ContainerResourceCaps:             nil,
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerResourceCaps: nil,
					DeploymentStrategy:    "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by adding container resource caps",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						ContainerResourceCaps: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("2Gi"),
								corev1.ResourceCPU:    resource.MustParse("2000m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
								corev1.ResourceCPU:    resource.MustParse("1000m"),
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
							ContainerResourceCaps: nil,
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerResourceCaps: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("2000m"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
							corev1.ResourceCPU:    resource.MustParse("1000m"),
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by changing container resource caps",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						ContainerResourceCaps: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("4Gi"),
								corev1.ResourceCPU:    resource.MustParse("4000m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("2Gi"),
								corev1.ResourceCPU:    resource.MustParse("2000m"),
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
							ContainerResourceCaps: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2Gi"),
									corev1.ResourceCPU:    resource.MustParse("2000m"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
									corev1.ResourceCPU:    resource.MustParse("1000m"),
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerResourceCaps: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
							corev1.ResourceCPU:    resource.MustParse("4000m"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("2000m"),
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by removing container resource caps",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						ContainerResourceCaps:             nil,
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
							ContainerResourceCaps: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2Gi"),
									corev1.ResourceCPU:    resource.MustParse("2000m"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
									corev1.ResourceCPU:    resource.MustParse("1000m"),
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ContainerResourceCaps: nil,
					DeploymentStrategy:    "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with per-user storage strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "15Gi",
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: ptr.To("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with per-workspace storage strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerWorkspacePVCStorageStrategy,
							PerWorkspaceStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "15Gi",
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: ptr.To("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						PerWorkspace: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update DevWorkspaceOperatorConfig with per-workspace storage strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerWorkspacePVCStorageStrategy,
							PerWorkspaceStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "15Gi",
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
							StorageClassName: ptr.To("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								PerWorkspace: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: ptr.To("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						PerWorkspace: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update DevWorkspaceOperatorConfig with per-user storage strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "15Gi",
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
							StorageClassName: ptr.To("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: ptr.To("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update populated DevWorkspaceOperatorConfig with storage class name, storage strategy and storage size",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: ptr.To(true),
						Storage: chev2.WorkspaceStorage{
							PvcStrategy: constants.PerUserPVCStorageStrategy,
							PerUserStrategyPvcConfig: &chev2.PVC{
								StorageClass: "test-storage",
								ClaimSize:    "15Gi",
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
						Routing: &controllerv1alpha1.RoutingConfig{
							DefaultRoutingClass: "routing-class",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Routing: &controllerv1alpha1.RoutingConfig{
					DefaultRoutingClass: "routing-class",
				},
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: ptr.To("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
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

			diff := cmp.Diff(testCase.expectedOperatorConfig, dwoc.Config, cmp.Options{
				cmpopts.IgnoreFields(controllerv1alpha1.WorkspaceConfig{}, "ServiceAccount"),
				cmpopts.IgnoreFields(controllerv1alpha1.RoutingConfig{}, "TLSCertificateConfigmapRef"),
			})
			assert.Empty(t, diff)
		})
	}

	for _, testCase := range expectedErrorTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)

			assert.Error(t, err)
			assert.Regexp(t, regexp.MustCompile(testCase.expectedErrorMessage), err.Error(), "error message must match")
		})
	}
}
