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
	"fmt"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	devWorkspaceConfigName = "devworkspace-config"
)

type DevWorkspaceConfigReconciler struct {
	deploy.Reconcilable
}

func NewDevWorkspaceConfigReconciler() *DevWorkspaceConfigReconciler {
	return &DevWorkspaceConfigReconciler{}
}

func (d *DevWorkspaceConfigReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devWorkspaceConfigName,
			Namespace: ctx.CheCluster.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DevWorkspaceOperatorConfig",
			APIVersion: controllerv1alpha1.GroupVersion.String(),
		},
	}

	if _, err := deploy.GetNamespacedObject(ctx, devWorkspaceConfigName, dwoc); err != nil {
		return reconcile.Result{}, false, err
	}

	if dwoc.Config == nil {
		dwoc.Config = &controllerv1alpha1.OperatorConfiguration{}
	}

	if err := updateWorkspaceConfig(ctx.CheCluster, dwoc.Config); err != nil {
		return reconcile.Result{}, false, err
	}

	if done, err := deploy.Sync(ctx, dwoc); !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (d *DevWorkspaceConfigReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func updateWorkspaceConfig(cheCluster *chev2.CheCluster, operatorConfig *controllerv1alpha1.OperatorConfiguration) error {
	devEnvironments := &cheCluster.Spec.DevEnvironments
	if operatorConfig.Workspace == nil {
		operatorConfig.Workspace = &controllerv1alpha1.WorkspaceConfig{}
	}

	if err := updateWorkspaceStorageConfig(devEnvironments, operatorConfig.Workspace); err != nil {
		return err
	}

	if err := updateWorkspaceServiceAccountConfig(devEnvironments, operatorConfig.Workspace); err != nil {
		return err
	}

	if err := updateWorkspacePodSchedulerNameConfig(devEnvironments, operatorConfig.Workspace); err != nil {
		return err
	}

	operatorConfig.Workspace.ContainerSecurityContext = nil
	if cheCluster.IsContainerBuildCapabilitiesEnabled() {
		operatorConfig.Workspace.ContainerSecurityContext = constants.DefaultWorkspaceContainerSecurityContext.DeepCopy()
	}

	updateStartTimeout(operatorConfig, devEnvironments.StartTimeoutSeconds)
	return nil
}

func updateStartTimeout(operatorConfig *controllerv1alpha1.OperatorConfiguration, startTimeoutSeconds *int32) {
	if startTimeoutSeconds == nil {
		// Allow the default start timeout of 5 minutes to be used if devEnvironments.StartTimeoutSeconds is unset
		operatorConfig.Workspace.ProgressTimeout = ""
	} else {
		operatorConfig.Workspace.ProgressTimeout = fmt.Sprintf("%ds", *startTimeoutSeconds)
	}
}

func updateWorkspaceStorageConfig(devEnvironments *chev2.CheClusterDevEnvironments, workspaceConfig *controllerv1alpha1.WorkspaceConfig) error {
	pvcStrategy := utils.GetValue(devEnvironments.Storage.PvcStrategy, constants.DefaultPvcStorageStrategy)
	isPerWorkspacePVCStorageStrategy := pvcStrategy == constants.PerWorkspacePVCStorageStrategy
	pvc := map[string]*chev2.PVC{
		constants.PerUserPVCStorageStrategy:      devEnvironments.Storage.PerUserStrategyPvcConfig,
		constants.CommonPVCStorageStrategy:       devEnvironments.Storage.PerUserStrategyPvcConfig,
		constants.PerWorkspacePVCStorageStrategy: devEnvironments.Storage.PerWorkspaceStrategyPvcConfig,
	}[pvcStrategy]

	if pvc != nil {
		if pvc.StorageClass != "" {
			workspaceConfig.StorageClassName = &pvc.StorageClass
		}

		if pvc.ClaimSize != "" {
			if workspaceConfig.DefaultStorageSize == nil {
				workspaceConfig.DefaultStorageSize = &controllerv1alpha1.StorageSizes{}
			}

			pvcSize, err := resource.ParseQuantity(pvc.ClaimSize)
			if err != nil {
				return err
			}

			if isPerWorkspacePVCStorageStrategy {
				workspaceConfig.DefaultStorageSize.PerWorkspace = &pvcSize
			} else {
				workspaceConfig.DefaultStorageSize.Common = &pvcSize
			}
		}
	}
	return nil
}

func updateWorkspaceServiceAccountConfig(devEnvironments *chev2.CheClusterDevEnvironments, workspaceConfig *controllerv1alpha1.WorkspaceConfig) error {
	isNamespaceAutoProvisioned := pointer.BoolPtrDerefOr(devEnvironments.DefaultNamespace.AutoProvision, constants.DefaultAutoProvision)

	workspaceConfig.ServiceAccount = &controllerv1alpha1.ServiceAccountConfig{
		ServiceAccountName:   devEnvironments.ServiceAccount,
		ServiceAccountTokens: devEnvironments.ServiceAccountTokens,
		// If user's Namespace is not auto provisioned (is pre-created by admin), then ServiceAccount must be pre-created as well
		DisableCreation: pointer.BoolPtr(!isNamespaceAutoProvisioned && devEnvironments.ServiceAccount != ""),
	}
	return nil
}

func updateWorkspacePodSchedulerNameConfig(devEnvironments *chev2.CheClusterDevEnvironments, workspaceConfig *controllerv1alpha1.WorkspaceConfig) error {
	workspaceConfig.SchedulerName = devEnvironments.PodSchedulerName
	return nil
}
