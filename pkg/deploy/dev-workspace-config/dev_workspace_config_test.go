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
package devworkspaceConfig

import (
	"os"
	"testing"

	"context"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type testCase struct {
	name           string
	infrastructure infrastructure.Type
	cheCluster     *chev2.CheCluster
}

var testCases = []testCase{
	{
		name: "Reconcile DevWorkspace Configuration on OpenShift",
		cheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		infrastructure: infrastructure.OpenShiftv4,
	},
	{
		name: "Reconcile DevWorkspace Configuration on K8S",
		cheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					CheServer: chev2.CheServer{
						ExtraProperties: map[string]string{"CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S": "true"},
					},
				},
				Networking: chev2.CheClusterSpecNetworking{
					Domain: "che.domain",
				},
			},
		},
		infrastructure: infrastructure.Kubernetes,
	},
}

func TestReconcileDevWorkspaceConfigPerUserStorage(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster.DeepCopy(), []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(controllerv1alpha1.SchemeBuilder.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{})

			dwoc := controllerv1alpha1.DevWorkspaceOperatorConfig{}
			namespace := deployContext.CheCluster.Namespace

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy = perUserStorageStrategy
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig = &chev2.PVC{}

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass = "testStorageClass"
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize = "15Gi"

			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err = devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			_, done, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace configuration has not been provisioned")

			// A DWOC should have now been created
			namespacedName := types.NamespacedName{Name: DwocName, Namespace: namespace}
			deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, &dwoc)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass, *dwoc.Config.Workspace.StorageClassName)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize, dwoc.Config.Workspace.DefaultStorageSize.Common.String())
		})
	}
}

func TestReconcileDevWorkspaceConfigPerWorkspaceStorage(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster.DeepCopy(), []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(controllerv1alpha1.SchemeBuilder.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{})

			dwoc := controllerv1alpha1.DevWorkspaceOperatorConfig{}
			namespace := deployContext.CheCluster.Namespace

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy = perWorkspaceStorageStrategy
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig = &chev2.PVC{}

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass = "testStorageClass"
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize = "15Gi"

			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err = devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			_, done, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace configuration has not been provisioned")

			// A DWOC should have now been created
			namespacedName := types.NamespacedName{Name: DwocName, Namespace: namespace}
			deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, &dwoc)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass, *dwoc.Config.Workspace.StorageClassName)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize, dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace.String())
		})
	}
}

func TestUpdateExistingEmptyDevWorkspaceConfigPerWorkspaceStorage(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster.DeepCopy(), []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(controllerv1alpha1.SchemeBuilder.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{})

			// Create DWOC which will be updated during reconcile
			dwoc := controllerv1alpha1.DevWorkspaceOperatorConfig{}
			namespace := deployContext.CheCluster.Namespace

			dwoc.ObjectMeta.Name = DwocName
			dwoc.ObjectMeta.Namespace = namespace
			deployContext.ClusterAPI.Client.Create(context.TODO(), &dwoc)

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy = perWorkspaceStorageStrategy
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig = &chev2.PVC{}
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass = "testStorageClass"
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize = "15Gi"

			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err = devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			_, done, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace configuration has not been provisioned")

			namespacedName := types.NamespacedName{Name: DwocName, Namespace: namespace}
			deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, &dwoc)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass, *dwoc.Config.Workspace.StorageClassName)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize, dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace.String())
		})
	}
}

func TestUpdateExistingEmptyDevWorkspaceConfigPerUserStorage(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster.DeepCopy(), []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(controllerv1alpha1.SchemeBuilder.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{})

			// Create DWOC which will be updated during reconcile
			dwoc := controllerv1alpha1.DevWorkspaceOperatorConfig{}
			namespace := deployContext.CheCluster.Namespace

			dwoc.ObjectMeta.Name = DwocName
			dwoc.ObjectMeta.Namespace = namespace
			deployContext.ClusterAPI.Client.Create(context.TODO(), &dwoc)

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy = perUserStorageStrategy
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig = &chev2.PVC{}
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass = "testStorageClass"
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize = "15Gi"

			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err = devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			_, done, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace configuration has not been provisioned")

			namespacedName := types.NamespacedName{Name: DwocName, Namespace: namespace}
			deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, &dwoc)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass, *dwoc.Config.Workspace.StorageClassName)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize, dwoc.Config.Workspace.DefaultStorageSize.Common.String())
		})
	}
}

func TestUpdateExistingPopulatedDevWorkspaceConfig(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster.DeepCopy(), []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(controllerv1alpha1.SchemeBuilder.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{})

			// Create DWOC which will be updated during reconcile
			dwoc := controllerv1alpha1.DevWorkspaceOperatorConfig{}
			namespace := deployContext.CheCluster.Namespace

			dwoc.ObjectMeta.Name = DwocName
			dwoc.ObjectMeta.Namespace = namespace
			dwoc.Config = &controllerv1alpha1.OperatorConfiguration{}
			dwoc.Config.Workspace = &controllerv1alpha1.WorkspaceConfig{}

			storageClass := "oldStorageClass"
			dwoc.Config.Workspace.StorageClassName = &storageClass

			commonStorageSize := resource.MustParse("12Gi")
			perWorkspaceStorageSize := resource.MustParse("13Gi")
			dwoc.Config.Workspace.DefaultStorageSize = &controllerv1alpha1.StorageSizes{
				Common:       &commonStorageSize,
				PerWorkspace: &perWorkspaceStorageSize,
			}

			deployContext.ClusterAPI.Client.Create(context.TODO(), &dwoc)

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy = commonStorageStrategy
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig = &chev2.PVC{
				ClaimSize:    "15Gi",
				StorageClass: "testStorageClass",
			}
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig = &chev2.PVC{
				ClaimSize: "15Gi",
			}
			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, _, err = devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			_, done, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace configuration has not been provisioned")

			namespacedName := types.NamespacedName{Name: DwocName, Namespace: namespace}
			deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, &dwoc)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass, *dwoc.Config.Workspace.StorageClassName)
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize, dwoc.Config.Workspace.DefaultStorageSize.Common.String())
			assert.Equal(t, deployContext.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize, dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace.String())
		})
	}
}

func TestNoDevWorkspaceConfigUpdateNecessary(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster.DeepCopy(), []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(controllerv1alpha1.SchemeBuilder.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{})

			// Create DWOC that will already be up to date on the cluster
			dwoc := controllerv1alpha1.DevWorkspaceOperatorConfig{}
			namespace := deployContext.CheCluster.Namespace

			dwoc.ObjectMeta.Name = DwocName
			dwoc.ObjectMeta.Namespace = namespace
			dwoc.Config = &controllerv1alpha1.OperatorConfiguration{}
			dwoc.Config.Workspace = &controllerv1alpha1.WorkspaceConfig{}
			storageClass := "testStorageClass"
			storageSize := resource.MustParse("9Gi")
			dwoc.Config.Workspace.StorageClassName = &storageClass
			dwoc.Config.Workspace.DefaultStorageSize = &controllerv1alpha1.StorageSizes{
				Common: &storageSize,
			}
			deployContext.ClusterAPI.Client.Create(context.TODO(), &dwoc)

			deployContext.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy = perUserStorageStrategy
			deployContext.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig = &chev2.PVC{
				ClaimSize:    storageSize.String(),
				StorageClass: "testStorageClass",
			}

			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			_, done, err := devWorkspaceConfigReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace configuration has not been provisioned")
		})
	}
}
