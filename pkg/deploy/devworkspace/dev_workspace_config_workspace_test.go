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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileDevWorkspaceProjectCloneConfig(t *testing.T) {
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
			runtimeDWOC := client.Object(existingDWOC)

			deployContext := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(runtimeDWOC).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testNamespace}, dwoc)
			assert.NoError(t, err)

			diff := cmp.Diff(testCase.expectedDevWorkspaceConfig, dwoc.Config.Workspace.ProjectCloneConfig)
			assert.Empty(t, diff)
		})
	}

}

func TestReconcileDevWorkspaceConfigPersistUserHome(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig when PersistUserHome is enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PersistUserHome: &chev2.PersistentHomeConfig{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig that does not have PersistUserHome config defined, when PersistUserHome is enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PersistUserHome: &chev2.PersistentHomeConfig{
							Enabled: ptr.To(true),
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
					PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		{
			name: "Set DevWorkspaceOperatorConfig PersistUserHome enabled when PersistUserHome is enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PersistUserHome: &chev2.PersistentHomeConfig{
							Enabled: ptr.To(true),
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
							PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
								Enabled: ptr.To(false),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		{
			name: "Set DevWorkspaceOperatorConfig PersistUserHome disableInitContainer to true",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PersistUserHome: &chev2.PersistentHomeConfig{
							Enabled:              ptr.To(true),
							DisableInitContainer: ptr.To(true),
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
							PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
								Enabled: ptr.To(false),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
						Enabled:              ptr.To(true),
						DisableInitContainer: ptr.To(true),
					},
				},
			},
		},
		{
			name: "Set DevWorkspaceOperatorConfig PersistUserHome disableInitContainer to false when initially true",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PersistUserHome: &chev2.PersistentHomeConfig{
							Enabled:              ptr.To(true),
							DisableInitContainer: ptr.To(false),
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
							PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
								Enabled:              ptr.To(true),
								DisableInitContainer: ptr.To(true),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
						Enabled:              ptr.To(true),
						DisableInitContainer: ptr.To(false),
					},
				},
			},
		},
		{
			name: "Set DevWorkspaceOperatorConfig PersistUserHome disabled when PersistUserHome is disabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						PersistUserHome: &chev2.PersistentHomeConfig{
							Enabled: ptr.To(false),
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
							PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
								Enabled: ptr.To(true),
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		{
			name: "Remove PersistUserHome config from existing DevWorkspaceOperatorConfig when PersistUserHome is removed from Che Cluster",
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
							PersistUserHome: &controllerv1alpha1.PersistentHomeConfig{
								Enabled: ptr.To(true),
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
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)
			assert.NoError(t, err)
			diff := cmp.Diff(testCase.expectedOperatorConfig, dwoc.Config, cmp.Options{
				cmpopts.IgnoreFields(controllerv1alpha1.WorkspaceConfig{}, "ServiceAccount", "DeploymentStrategy", "ContainerSecurityContext"),
				cmpopts.IgnoreFields(controllerv1alpha1.RoutingConfig{}, "TLSCertificateConfigmapRef"),
			})
			assert.Empty(t, diff)
		})
	}
}

func TestReconcileDevWorkspaceImagePullPolicy(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
		{
			name: "Set specific pull policy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ImagePullPolicy: corev1.PullAlways,
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ImagePullPolicy: string(corev1.PullAlways),
				},
			},
		},
		{
			name: "Clean up pull policy",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						ImagePullPolicy: "",
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ImagePullPolicy: "",
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
							ImagePullPolicy: string(corev1.PullAlways),
						},
					},
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
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.ImagePullPolicy, dwoc.Config.Workspace.ImagePullPolicy)
		})
	}
}

func TestReconcileDevWorkspaceAnnotations(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
		{
			name: "Set specific annotations",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						WorkspacesPodAnnotations: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PodAnnotations: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
		},
		{
			name: "Remove annotations",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						WorkspacesPodAnnotations: nil,
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					PodAnnotations: nil,
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
							PodAnnotations: map[string]string{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
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
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.PodAnnotations, dwoc.Config.Workspace.PodAnnotations)
		})
	}
}

func TestReconcileDevWorkspaceIgnoredUnrecoverableEvents(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
		{
			name: "Set events",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						IgnoredUnrecoverableEvents: []string{
							"value1",
							"value2",
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					IgnoredUnrecoverableEvents: []string{
						"value1",
						"value2",
					},
				},
			},
		},
		{
			name: "Remove events",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						IgnoredUnrecoverableEvents: nil,
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					IgnoredUnrecoverableEvents: nil,
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
							IgnoredUnrecoverableEvents: []string{
								"value1",
								"value2",
							},
						},
					},
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
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.IgnoredUnrecoverableEvents, dwoc.Config.Workspace.IgnoredUnrecoverableEvents)
		})
	}
}

func TestReconcileDevWorkspaceConfigForInitContainers(t *testing.T) {
	type testCase struct {
		name                   string
		cheCluster             *chev2.CheCluster
		existedObjects         []client.Object
		expectedOperatorConfig *controllerv1alpha1.OperatorConfiguration
	}

	testCases := []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with InitContainers",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						InitContainers: []corev1.Container{
							{
								Name:  "init-container-1",
								Image: "init-image:latest",
								Command: []string{
									"/bin/sh",
									"-c",
									"echo 'Initializing workspace'",
								},
							},
							{
								Name:  "init-container-2",
								Image: "init-image-2:v1.0",
								Env: []corev1.EnvVar{
									{
										Name:  "INIT_VAR",
										Value: "init-value",
									},
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					InitContainers: []corev1.Container{
						{
							Name:  "init-container-1",
							Image: "init-image:latest",
							Command: []string{
								"/bin/sh",
								"-c",
								"echo 'Initializing workspace'",
							},
						},
						{
							Name:  "init-container-2",
							Image: "init-image-2:v1.0",
							Env: []corev1.EnvVar{
								{
									Name:  "INIT_VAR",
									Value: "init-value",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Create DevWorkspaceOperatorConfig without InitContainers",
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
			name: "Update DevWorkspaceOperatorConfig with InitContainers",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						InitContainers: []corev1.Container{
							{
								Name:  "new-init-container",
								Image: "new-init:v2.0",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config-volume",
										MountPath: "/etc/config",
									},
								},
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
							InitContainers: []corev1.Container{
								{
									Name:  "old-init-container",
									Image: "old-init:v1.0",
								},
							},
						},
					},
				},
			},
			expectedOperatorConfig: &controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					InitContainers: []corev1.Container{
						{
							Name:  "new-init-container",
							Image: "new-init:v2.0",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/config",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Clear InitContainers from DevWorkspaceOperatorConfig",
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
							InitContainers: []corev1.Container{
								{
									Name:  "init-to-remove",
									Image: "init:v1.0",
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
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: devWorkspaceConfigName, Namespace: testCase.cheCluster.Namespace}, dwoc)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedOperatorConfig.Workspace.InitContainers, dwoc.Config.Workspace.InitContainers)
		})
	}
}
