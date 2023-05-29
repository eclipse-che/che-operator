//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspaceconfig

import (
	"context"
	"regexp"
	"testing"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestReconcileDevWorkspaceConfigPerUserStorage(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []runtime.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	type errorTestCase struct {
		name                 string
		cheCluster           *chev2.CheCluster
		existedObjects       []runtime.Object
		expectedErrorMessage string
	}

	var quantity15Gi = resource.MustParse("15Gi")
	var quantity10Gi = resource.MustParse("10Gi")
	var quantity1CPU = resource.MustParse("1000m")
	var quantity500mCPU = resource.MustParse("500m")

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

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{Workspace: &controllerv1alpha1.WorkspaceConfig{DeploymentStrategy: "Recreate"}},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with ephemeral strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
					StorageClassName:   pointer.String("test-storage"),
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with per-user strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
					StorageClassName: pointer.String("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with per-workspace strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
					StorageClassName: pointer.String("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						PerWorkspace: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update DevWorkspaceOperatorConfig with per-workspace strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								PerWorkspace: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						PerWorkspace: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update DevWorkspaceOperatorConfig with per-user strategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update populated DevWorkspaceOperatorConfig",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
			existedObjects: []runtime.Object{
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
						Workspace: &controllerv1alpha1.WorkspaceConfig{
							ImagePullPolicy: "Always",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Routing: &controllerv1alpha1.RoutingConfig{
					DefaultRoutingClass: "routing-class",
				},
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ImagePullPolicy:  "Always",
					StorageClassName: pointer.String("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig without Pod Security Context if container build disabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
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
			name: "Create DevWorkspaceOperatorConfig with Pod and Container Security Context if container build enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(false),
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
						AllowPrivilegeEscalation: pointer.Bool(true),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by adding Pod and Container Security Context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(false),
					},
				},
			},
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
					ContainerSecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SETGID",
								"SETUID",
							},
						},
						AllowPrivilegeEscalation: pointer.Bool(true),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by removing Pod and Container Security Context",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
					},
				},
			},
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
							ContainerSecurityContext: &corev1.SecurityContext{},
							DeploymentStrategy:       "Recreate",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with progressTimeout",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
						StartTimeoutSeconds:               pointer.Int32(600),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ProgressTimeout:    "600s",
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by adding progressTimeout",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
						StartTimeoutSeconds:               pointer.Int32(600),
					},
				},
			},
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
					ProgressTimeout:    "600s",
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by overwriting progressTimeout",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
						StartTimeoutSeconds:               pointer.Int32(420),
					},
				},
			},
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
							ProgressTimeout: "1h30m",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
					ProgressTimeout:    "420s",
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig by removing progressTimeout",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
					},
				},
			},
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
							ProgressTimeout: "1h30m",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Configures ProjectCloneConfig in DevWorkspaceOperatorConfig",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DisableContainerBuildCapabilities: pointer.Bool(true),
						ProjectCloneContainer: &chev2.Container{
							Name:            "project-clone",
							Image:           "test-image",
							ImagePullPolicy: "IfNotPresent",
							Env: []corev1.EnvVar{
								{Name: "test-env-1", Value: "test-val-1"},
								{Name: "test-env-2", Value: "test-val-2"},
							},
							Resources: &chev2.ResourceRequirements{
								Requests: &chev2.ResourceList{
									Memory: &quantity10Gi,
									Cpu:    &quantity500mCPU,
								},
								Limits: &chev2.ResourceList{
									Memory: &quantity15Gi,
									Cpu:    &quantity1CPU,
								},
							},
						},
					},
				},
			},
			existedObjects: []runtime.Object{
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
							StorageClassName: pointer.String("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
							ProgressTimeout: "1h30m",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.String("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
					DeploymentStrategy: "Recreate",
					ProjectCloneConfig: &controllerv1alpha1.ProjectCloneConfig{
						Image:           "test-image",
						ImagePullPolicy: "IfNotPresent",
						Env: []corev1.EnvVar{
							{Name: "test-env-1", Value: "test-val-1"},
							{Name: "test-env-2", Value: "test-val-2"},
						},
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: quantity10Gi,
								corev1.ResourceCPU:    quantity500mCPU,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: quantity15Gi,
								corev1.ResourceCPU:    quantity1CPU,
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, testCase.existedObjects)
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)

			diff := cmp.Diff(testCase.expectedOperatorConfig, dwoc.Config, cmp.Options{cmpopts.IgnoreFields(controllerv1alpha1.WorkspaceConfig{}, "ServiceAccount")})
			assert.Empty(t, diff)
		})
	}

	for _, testCase := range expectedErrorTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, testCase.existedObjects)
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.Error(t, err)
			assert.Regexp(t, regexp.MustCompile(testCase.expectedErrorMessage), err.Error(), "error message must match")
		})
	}
}

func TestReconcileServiceAccountConfig(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	var testCases = []testCase{
		{
			name: "Case #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ServiceAccount: "service-account",
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountName: "service-account",
						DisableCreation:    pointer.Bool(false),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Case #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultNamespace: chev2.DefaultNamespace{
							AutoProvision: pointer.Bool(false),
						},
						ServiceAccount: "service-account",
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountName: "service-account",
						DisableCreation:    pointer.Bool(true),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Case #3",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						DisableCreation: pointer.Bool(false),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Case #4",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultNamespace: chev2.DefaultNamespace{
							AutoProvision: pointer.Bool(false),
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						DisableCreation: pointer.Bool(false),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)

			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.ServiceAccount, dwoc.Config.Workspace.ServiceAccount)
		})
	}
}

func TestReconcileDevWorkspaceConfigPodSchedulerName(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []runtime.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with podSchedulerName",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PodSchedulerName: "test-scheduler",
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					SchedulerName:      "test-scheduler",
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when PodSchedulerName is added",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PodSchedulerName: "test-scheduler",
					},
				},
			},
			existedObjects: []runtime.Object{
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
					SchedulerName:      "test-scheduler",
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when PodSchedulerName is changed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PodSchedulerName: "test-scheduler",
					},
				},
			},
			existedObjects: []runtime.Object{
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
							SchedulerName: "previous-scheduler",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					SchedulerName:      "test-scheduler",
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when PodSchedulerName is removed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PodSchedulerName: "",
					},
				},
			},
			existedObjects: []runtime.Object{
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
							SchedulerName: "previous-scheduler",
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.SchedulerName, dwoc.Config.Workspace.SchedulerName)
		})
	}
}

func TestReconcileDevWorkspaceConfigServiceAccountTokens(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []runtime.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with ServiceAccountTokens",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
							{
								Name:              "test-token-1",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "openshift",
								ExpirationSeconds: 3600,
							},
							{
								Name:              "test-token-2",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "kubernetes",
								ExpirationSeconds: 180,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
							{
								Name:              "test-token-1",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "openshift",
								ExpirationSeconds: 3600,
							},
							{
								Name:              "test-token-2",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "kubernetes",
								ExpirationSeconds: 180,
							},
						}},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when ServiceAccountTokens are added",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
							{
								Name:              "test-token",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "openshift",
								ExpirationSeconds: 3600,
							},
							{
								Name:              "test-token-2",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "kubernetes",
								ExpirationSeconds: 180,
							},
						},
					},
				},
			},
			existedObjects: []runtime.Object{
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
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
							{
								Name:              "test-token",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "openshift",
								ExpirationSeconds: 3600,
							},
							{
								Name:              "test-token-2",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "kubernetes",
								ExpirationSeconds: 180,
							},
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when ServiceAccountTokens are changed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
							{
								Name:              "new-token",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "openshift",
								ExpirationSeconds: 3600,
							},
						},
					},
				},
			},
			existedObjects: []runtime.Object{
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
							ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
								ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
									{
										Name:              "old-token",
										MountPath:         "/var/run/secrets/tokens",
										Audience:          "openshift",
										ExpirationSeconds: 60,
									},
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
							{
								Name:              "new-token",
								MountPath:         "/var/run/secrets/tokens",
								Audience:          "openshift",
								ExpirationSeconds: 3600,
							},
						},
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when ServiceAccountTokens are removed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{},
					},
				},
			},
			existedObjects: []runtime.Object{
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
							ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
								ServiceAccountTokens: []controllerv1alpha1.ServiceAccountToken{
									{
										Name:              "test-token",
										MountPath:         "/var/run/secrets/tokens",
										Audience:          "openshift",
										ExpirationSeconds: 3600,
									},
									{
										Name:              "test-token-2",
										MountPath:         "/var/run/secrets/tokens",
										Audience:          "kubernetes",
										ExpirationSeconds: 180,
									},
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount:     &controllerv1alpha1.ServiceAccountConfig{},
					DeploymentStrategy: "Recreate",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.ServiceAccount.ServiceAccountTokens, dwoc.Config.Workspace.ServiceAccount.ServiceAccountTokens)
		})
	}
}

func TestReconcileDevWorkspaceConfigDeploymentStrategy(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []runtime.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with DeploymentStrategy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DeploymentStrategy: "Recreate",
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
			name: "Update existing DevWorkspaceOperatorConfig when DeploymentStrategy is added",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DeploymentStrategy: "Recreate",
					},
				},
			},
			existedObjects: []runtime.Object{
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
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig when DeploymentStrategy is changed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DeploymentStrategy: "RollingUpdate",
					},
				},
			},
			existedObjects: []runtime.Object{
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
							DeploymentStrategy: "Recreate",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					DeploymentStrategy: "RollingUpdate",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.DeploymentStrategy, dwoc.Config.Workspace.DeploymentStrategy)
		})
	}
}

func TestReconcileDevWorkspaceProjectCloneCOnfig(t *testing.T) {
	const testNamespace = "eclipse-che"
	testMemLimit := resource.MustParse("2Gi")
	testCpuLimit := resource.MustParse("1000m")
	testMemRequest := resource.MustParse("1Gi")
	testCpuRequest := resource.MustParse("500m")

	type testCase struct {
		name                       string
		cheProjectCloneConfig      *chev2.Container
		expectedDevWorkspaceConfig *controllerv1alpha1.ProjectCloneConfig
		existingDevWorkspaceConfig *controllerv1alpha1.ProjectCloneConfig
	}

	tests := []testCase{
		{
			name: "Syncs Che project clone config to DevWorkspaceOperatorConfig",
			cheProjectCloneConfig: &chev2.Container{
				Name:            "project-clone",
				Image:           "test-image",
				ImagePullPolicy: "IfNotPresent",
				Env: []corev1.EnvVar{
					{Name: "test-env-1", Value: "test-val-1"},
					{Name: "test-env-2", Value: "test-val-2"},
				},
				Resources: &chev2.ResourceRequirements{
					Limits: &chev2.ResourceList{
						Memory: &testMemLimit,
						Cpu:    &testCpuLimit,
					},
					Requests: &chev2.ResourceList{
						Memory: &testMemRequest,
						Cpu:    &testCpuRequest,
					},
				},
			},
			expectedDevWorkspaceConfig: &controllerv1alpha1.ProjectCloneConfig{
				Image:           "test-image",
				ImagePullPolicy: "IfNotPresent",
				Env: []corev1.EnvVar{
					{Name: "test-env-1", Value: "test-val-1"},
					{Name: "test-env-2", Value: "test-val-2"},
				},
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: testMemLimit,
						corev1.ResourceCPU:    testCpuLimit,
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: testMemRequest,
						corev1.ResourceCPU:    testCpuRequest,
					},
				},
			},
		},
		{
			name: "Updates existing DevWorkspaceOperatorConfig with new Che project clone config",
			cheProjectCloneConfig: &chev2.Container{
				Name:            "project-clone",
				Image:           "test-image",
				ImagePullPolicy: "IfNotPresent",
				Env: []corev1.EnvVar{
					{Name: "test-env-1", Value: "test-val-1"},
					{Name: "test-env-2", Value: "test-val-2"},
				},
				Resources: &chev2.ResourceRequirements{
					Limits: &chev2.ResourceList{
						Memory: &testMemLimit,
						Cpu:    &testCpuLimit,
					},
					Requests: &chev2.ResourceList{
						Memory: &testMemRequest,
						Cpu:    &testCpuRequest,
					},
				},
			},
			expectedDevWorkspaceConfig: &controllerv1alpha1.ProjectCloneConfig{
				Image:           "test-image",
				ImagePullPolicy: "IfNotPresent",
				Env: []corev1.EnvVar{
					{Name: "test-env-1", Value: "test-val-1"},
					{Name: "test-env-2", Value: "test-val-2"},
				},
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: testMemLimit,
						corev1.ResourceCPU:    testCpuLimit,
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: testMemRequest,
						corev1.ResourceCPU:    testCpuRequest,
					},
				},
			},
			existingDevWorkspaceConfig: &controllerv1alpha1.ProjectCloneConfig{
				Image:           "other image",
				ImagePullPolicy: "Always",
				Env: []corev1.EnvVar{
					{Name: "other-env", Value: "other-val"},
				},
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1234Mi"),
						corev1.ResourceCPU:    resource.MustParse("1234m"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1111Mi"),
						corev1.ResourceCPU:    resource.MustParse("1111m"),
					},
				},
			},
		},
		{
			name: "Removes fields from existing config when removed from CheCluster",
			cheProjectCloneConfig: &chev2.Container{
				Name:            "",
				Image:           "",
				ImagePullPolicy: "",
				Env:             nil,
				Resources:       nil,
			},
			expectedDevWorkspaceConfig: &controllerv1alpha1.ProjectCloneConfig{
				Image:           "",
				ImagePullPolicy: "",
				Env:             nil,
				Resources:       nil,
			},
			existingDevWorkspaceConfig: &controllerv1alpha1.ProjectCloneConfig{
				Image:           "other image",
				ImagePullPolicy: "Always",
				Env: []corev1.EnvVar{
					{Name: "other-env", Value: "other-val"},
				},
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1234Mi"),
						corev1.ResourceCPU:    resource.MustParse("1234m"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1111Mi"),
						corev1.ResourceCPU:    resource.MustParse("1111m"),
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cheCluster := &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ProjectCloneContainer: testCase.cheProjectCloneConfig,
					},
				},
			}
			existingDWOC := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      devWorkspaceConfigName,
					Namespace: testNamespace,
				},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						ProjectCloneConfig: testCase.existingDevWorkspaceConfig,
					},
				},
			}
			runtimeDWOC := runtime.Object(existingDWOC)

			deployContext := test.GetDeployContext(cheCluster, []runtime.Object{runtimeDWOC})
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testNamespace}, dwoc)
			assert.NoError(t, err)

			diff := cmp.Diff(testCase.expectedDevWorkspaceConfig, dwoc.Config.Workspace.ProjectCloneConfig)
			assert.Empty(t, diff)
		})
	}

}
