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
	"context"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const DwocName string = "devworkspace-config"
const perUserStorageStrategy string = "per-user"
const commonStorageStrategy string = "common"
const perWorkspaceStorageStrategy string = "per-workspace"

type DevWorkspaceConfigReconciler struct {
	deploy.Reconcilable
}

func NewDevWorkspaceConfigReconciler() *DevWorkspaceConfigReconciler {
	return &DevWorkspaceConfigReconciler{}
}

func (d *DevWorkspaceConfigReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// Get the che-operator-owned DWOC, if it exists. Otherwise, create it.
	namespace := ctx.CheCluster.Namespace
	dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
	namespacedName := types.NamespacedName{Name: DwocName, Namespace: namespace}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), namespacedName, dwoc)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return reconcile.Result{}, false, createDWOC(dwoc, ctx, namespace)
		}
		return reconcile.Result{}, false, err
	}

	// Now that we have the DWOC, modify it accordingly
	updatedConfig, err := updateDWOC(ctx, dwoc)

	if err != nil {
		return reconcile.Result{}, false, err
	}

	if updatedConfig {
		// Now sync the updated config with the cluster
		didSync, err := deploy.Sync(ctx, dwoc)
		if err != nil {
			return reconcile.Result{}, false, err
		}

		if !didSync {
			return reconcile.Result{Requeue: true}, false, nil
		}
	}

	return reconcile.Result{}, true, nil
}

func (d *DevWorkspaceConfigReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func updateDWOC(ctx *chetypes.DeployContext, dwoc *controllerv1alpha1.DevWorkspaceOperatorConfig) (bool, error) {
	updatedConfig := false

	if dwoc.Config == nil {
		dwoc.Config = &controllerv1alpha1.OperatorConfiguration{}
		updatedConfig = true
	}

	if dwoc.Config.Workspace == nil {
		dwoc.Config.Workspace = &controllerv1alpha1.WorkspaceConfig{}
		updatedConfig = true
	}

	// Setting the storage class name requires that a PVC strategy be selected
	if ctx.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy != "" {

		switch ctx.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy {
		case commonStorageStrategy:
			fallthrough
		case perUserStorageStrategy:

			if ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig != nil &&
				ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass != "" {

				if dwoc.Config.Workspace.StorageClassName == nil ||
					ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass != *dwoc.Config.Workspace.StorageClassName {
					dwoc.Config.Workspace.StorageClassName = &ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass
					updatedConfig = true
				}
			}
		case perWorkspaceStorageStrategy:
			if ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig != nil &&
				ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass != "" {

				if dwoc.Config.Workspace.StorageClassName == nil ||
					ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass != *dwoc.Config.Workspace.StorageClassName {
					dwoc.Config.Workspace.StorageClassName = &ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass
					updatedConfig = true
				}

			}
		}
	}

	if ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig != nil &&
		ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize != "" {

		if dwoc.Config.Workspace.DefaultStorageSize == nil {
			dwoc.Config.Workspace.DefaultStorageSize = &controllerv1alpha1.StorageSizes{}
			updatedConfig = true
		}

		if dwoc.Config.Workspace.DefaultStorageSize.Common == nil ||
			dwoc.Config.Workspace.DefaultStorageSize.Common.String() != ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize {

			pvcSize, err := resource.ParseQuantity(ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize)
			if err != nil {
				return updatedConfig, err
			}
			dwoc.Config.Workspace.DefaultStorageSize.Common = &pvcSize
			updatedConfig = true
		}

	}

	if ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig != nil &&
		ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize != "" {

		if dwoc.Config.Workspace.DefaultStorageSize == nil {
			dwoc.Config.Workspace.DefaultStorageSize = &controllerv1alpha1.StorageSizes{}
			updatedConfig = true
		}

		if dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace == nil ||
			dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace.String() != ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize {

			pvcSize, err := resource.ParseQuantity(ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize)
			if err != nil {
				return updatedConfig, err
			}
			dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace = &pvcSize
			updatedConfig = true
		}

	}

	return updatedConfig, nil
}

func createDWOC(dwoc *controllerv1alpha1.DevWorkspaceOperatorConfig, ctx *chetypes.DeployContext, namespace string) error {
	dwoc.ObjectMeta.Namespace = namespace
	dwoc.ObjectMeta.Name = DwocName
	dwoc.Config = &controllerv1alpha1.OperatorConfiguration{}
	dwoc.Config.Workspace = &controllerv1alpha1.WorkspaceConfig{}

	// Setting the storage class name requires that a PVC strategy be selected
	if ctx.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy != "" {
		switch ctx.CheCluster.Spec.DevEnvironments.Storage.PvcStrategy {
		case commonStorageStrategy:
			fallthrough
		case perUserStorageStrategy:
			if ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig != nil && ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass != "" {
				dwoc.Config.Workspace.StorageClassName = &ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass
			}
		case perWorkspaceStorageStrategy:
			if ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig != nil && ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass != "" {
				dwoc.Config.Workspace.StorageClassName = &ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass
			}
		}
	}

	if ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig != nil && ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize != "" {
		pvcSize, err := resource.ParseQuantity(ctx.CheCluster.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize)
		if err != nil {
			return err
		}

		dwoc.Config.Workspace.DefaultStorageSize = &controllerv1alpha1.StorageSizes{}
		dwoc.Config.Workspace.DefaultStorageSize.Common = &pvcSize
	}

	if ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig != nil && ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize != "" {
		pvcSize, err := resource.ParseQuantity(ctx.CheCluster.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize)
		if err != nil {
			return err
		}

		if dwoc.Config.Workspace.DefaultStorageSize == nil {
			dwoc.Config.Workspace.DefaultStorageSize = &controllerv1alpha1.StorageSizes{}
		}
		dwoc.Config.Workspace.DefaultStorageSize.PerWorkspace = &pvcSize
	}

	return ctx.ClusterAPI.Client.Create(context.TODO(), dwoc)
}
