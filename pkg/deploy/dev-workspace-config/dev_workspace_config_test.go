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
	"regexp"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	"context"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
					DevEnvironments: chev2.CheClusterDevEnvironments{},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
			},
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
					StorageClassName: pointer.StringPtr("test-storage"),
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
					StorageClassName: pointer.StringPtr("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
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
					StorageClassName: pointer.StringPtr("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						PerWorkspace: &quantity15Gi,
					},
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
							StorageClassName: pointer.StringPtr("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								PerWorkspace: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.StringPtr("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						PerWorkspace: &quantity15Gi,
					},
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
							StorageClassName: pointer.StringPtr("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.StringPtr("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
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
							ProgressTimeout: "10s",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Routing: &controllerv1alpha1.RoutingConfig{
					DefaultRoutingClass: "routing-class",
				},
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ProgressTimeout:  "10s",
					StorageClassName: pointer.StringPtr("test-storage"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity15Gi,
					},
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
						DisableContainerBuildCapabilities: pointer.BoolPtr(true),
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
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
						DisableContainerBuildCapabilities: pointer.BoolPtr(false),
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
						AllowPrivilegeEscalation: pointer.BoolPtr(false),
					},
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
						DisableContainerBuildCapabilities: pointer.BoolPtr(false),
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
							StorageClassName: pointer.StringPtr("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.StringPtr("default-storage-class"),
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
						AllowPrivilegeEscalation: pointer.BoolPtr(false),
					},
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
						DisableContainerBuildCapabilities: pointer.BoolPtr(true),
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
							StorageClassName: pointer.StringPtr("default-storage-class"),
							DefaultStorageSize: &controllerv1alpha1.StorageSizes{
								Common: &quantity10Gi,
							},
							ContainerSecurityContext: &corev1.SecurityContext{},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					StorageClassName: pointer.StringPtr("default-storage-class"),
					DefaultStorageSize: &controllerv1alpha1.StorageSizes{
						Common: &quantity10Gi,
					},
				},
			},
		},
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
					SchedulerName: "test-scheduler",
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
					SchedulerName: "test-scheduler",
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
					SchedulerName: "test-scheduler",
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
				Workspace: &controllerv1alpha1.WorkspaceConfig{},
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

			assert.Equal(t, testCase.expectedOperatorConfig, dwoc.Config)
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
