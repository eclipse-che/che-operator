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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

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
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{Workspace: &controllerv1alpha1.WorkspaceConfig{}},
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
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{Workspace: &controllerv1alpha1.WorkspaceConfig{}},
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
						DisableCreation:    pointer.BoolPtr(false),
					},
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
							AutoProvision: pointer.BoolPtr(false),
						},
						ServiceAccount: "service-account",
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountName: "service-account",
						DisableCreation:    pointer.BoolPtr(true),
					},
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
						DisableCreation: pointer.BoolPtr(false),
					},
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
							AutoProvision: pointer.BoolPtr(false),
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						DisableCreation: pointer.BoolPtr(false),
					},
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
