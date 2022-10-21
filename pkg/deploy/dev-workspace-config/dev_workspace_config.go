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
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	err := updateOperatorConfig(ctx.CheCluster, dwoc.Config)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	done, err := deploy.Sync(ctx, dwoc)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (d *DevWorkspaceConfigReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func updateOperatorConfig(cheCluster *chev2.CheCluster, operatorConfig *controllerv1alpha1.OperatorConfiguration) error {
	var pvc *chev2.PVC
	storage := cheCluster.Spec.DevEnvironments.Storage

	pvcStrategy := utils.GetValue(storage.PvcStrategy, constants.DefaultPvcStorageStrategy)
	switch pvcStrategy {
	case constants.CommonPVCStorageStrategy:
		fallthrough
	case constants.PerUserPVCStorageStrategy:
		if storage.PerUserStrategyPvcConfig != nil {
			pvc = storage.PerUserStrategyPvcConfig
		}
	case constants.PerWorkspacePVCStorageStrategy:
		if storage.PerWorkspaceStrategyPvcConfig != nil {
			pvc = storage.PerWorkspaceStrategyPvcConfig
		}
	}

	if operatorConfig.Workspace == nil {
		operatorConfig.Workspace = &controllerv1alpha1.WorkspaceConfig{}
	}

	return updateWorkspaceConfig(pvc, pvcStrategy == constants.PerWorkspacePVCStorageStrategy, cheCluster.IsContainerBuildCapabilitiesEnabled(), operatorConfig.Workspace)
}

func updateWorkspaceConfig(pvc *chev2.PVC, isPerWorkspacePVCStorageStrategy bool, enabledContainerBuildCapabilities bool, workspaceConfig *controllerv1alpha1.WorkspaceConfig) error {
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

	workspaceConfig.ContainerSecurityContext = nil
	if enabledContainerBuildCapabilities {
		workspaceConfig.ContainerSecurityContext = constants.DefaultWorkspaceContainerSecurityContext.DeepCopy()
	}

	return nil
}
