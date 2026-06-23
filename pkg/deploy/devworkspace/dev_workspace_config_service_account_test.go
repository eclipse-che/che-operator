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
	"testing"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileServiceAccountConfig(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with ServiceAccount name and default auto-provision",
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
						DisableCreation:    ptr.To(false),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with ServiceAccount name and auto-provision disabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultNamespace: chev2.DefaultNamespace{
							AutoProvision: ptr.To(false),
						},
						ServiceAccount: "service-account",
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						ServiceAccountName: "service-account",
						DisableCreation:    ptr.To(true),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with default ServiceAccount config when no service account specified",
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
						DisableCreation: ptr.To(false),
					},
					DeploymentStrategy: "Recreate",
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig with default ServiceAccount config when auto-provision disabled without service account",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultNamespace: chev2.DefaultNamespace{
							AutoProvision: ptr.To(false),
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ServiceAccount: &controllerv1alpha1.ServiceAccountConfig{
						DisableCreation: ptr.To(false),
					},
					DeploymentStrategy: "Recreate",
				},
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

			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.ServiceAccount, dwoc.Config.Workspace.ServiceAccount)
		})
	}
}

func TestReconcileDevWorkspaceConfigServiceAccountTokens(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
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
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.ServiceAccount.ServiceAccountTokens, dwoc.Config.Workspace.ServiceAccount.ServiceAccountTokens)
		})
	}
}
