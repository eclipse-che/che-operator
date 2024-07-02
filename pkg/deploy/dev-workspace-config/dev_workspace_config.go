//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"encoding/json"
	"fmt"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	if err := updateWorkspaceConfig(ctx, dwoc.Config); err != nil {
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

func updateWorkspaceConfig(ctx *chetypes.DeployContext, operatorConfig *controllerv1alpha1.OperatorConfiguration) error {
	cheCluster := ctx.CheCluster
	devEnvironments := &cheCluster.Spec.DevEnvironments
	if operatorConfig.Workspace == nil {
		operatorConfig.Workspace = &controllerv1alpha1.WorkspaceConfig{}
	}

	if err := updateWorkspaceStorageConfig(devEnvironments, operatorConfig.Workspace); err != nil {
		return err
	}

	updateWorkspaceServiceAccountConfig(devEnvironments, operatorConfig.Workspace)

	updateWorkspacePodSchedulerNameConfig(devEnvironments, operatorConfig.Workspace)

	updateProjectCloneConfig(devEnvironments, operatorConfig.Workspace)

	if err := updateSecurityContext(operatorConfig, cheCluster); err != nil {
		return err
	}

	updateStartTimeout(operatorConfig, devEnvironments.StartTimeoutSeconds)

	updatePersistUserHomeConfig(devEnvironments.PersistUserHome, operatorConfig.Workspace)

	updateWorkspaceImagePullPolicy(devEnvironments.ImagePullPolicy, operatorConfig.Workspace)

	updateAnnotations(devEnvironments.WorkspacesPodAnnotations, operatorConfig.Workspace)

	updateIgnoredUnrecoverableEvents(devEnvironments.IgnoredUnrecoverableEvents, operatorConfig.Workspace)

	// If the CheCluster has a configured proxy, or if the Che Operator has detected a proxy configuration,
	// we need to disable automatic proxy handling in the DevWorkspace Operator as its implementation collides
	// with ours -- they set environment variables the deployment spec explicitly, which overrides the proxy-settings
	// automount configmap.
	if ctx.Proxy.HttpProxy != "" || ctx.Proxy.HttpsProxy != "" {
		if operatorConfig.Routing == nil {
			operatorConfig.Routing = &controllerv1alpha1.RoutingConfig{}
		}
		disableDWOProxy(operatorConfig.Routing)
	}

	operatorConfig.Workspace.DeploymentStrategy = v1.DeploymentStrategyType(utils.GetValue(string(devEnvironments.DeploymentStrategy), constants.DefaultDeploymentStrategy))
	return nil
}

func updateSecurityContext(operatorConfig *controllerv1alpha1.OperatorConfiguration, cheCluster *chev2.CheCluster) error {
	operatorConfig.Workspace.ContainerSecurityContext = nil
	if cheCluster.IsContainerBuildCapabilitiesEnabled() {
		defaultContainerSecurityContext, err := getDefaultContainerSecurityContext()
		if err != nil {
			return err
		}
		operatorConfig.Workspace.ContainerSecurityContext = defaultContainerSecurityContext
	} else if cheCluster.Spec.DevEnvironments.Security.ContainerSecurityContext != nil {
		operatorConfig.Workspace.ContainerSecurityContext = cheCluster.Spec.DevEnvironments.Security.ContainerSecurityContext
	}

	operatorConfig.Workspace.PodSecurityContext = nil
	if cheCluster.Spec.DevEnvironments.Security.PodSecurityContext != nil {
		operatorConfig.Workspace.PodSecurityContext = cheCluster.Spec.DevEnvironments.Security.PodSecurityContext
	}
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

func updatePersistUserHomeConfig(persistentHomeConfig *chev2.PersistentHomeConfig, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	workspaceConfig.PersistUserHome = nil
	if persistentHomeConfig != nil {
		workspaceConfig.PersistUserHome = &controllerv1alpha1.PersistentHomeConfig{}
		workspaceConfig.PersistUserHome.Enabled = persistentHomeConfig.Enabled
	}
}

func updateWorkspaceImagePullPolicy(imagePullPolicy corev1.PullPolicy, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	workspaceConfig.ImagePullPolicy = string(imagePullPolicy)
}

func updateAnnotations(annotations map[string]string, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	workspaceConfig.PodAnnotations = annotations
}

func updateIgnoredUnrecoverableEvents(ignoredUnrecoverableEvents []string, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	workspaceConfig.IgnoredUnrecoverableEvents = ignoredUnrecoverableEvents
}

func updateWorkspaceServiceAccountConfig(devEnvironments *chev2.CheClusterDevEnvironments, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	isNamespaceAutoProvisioned := pointer.BoolDeref(devEnvironments.DefaultNamespace.AutoProvision, constants.DefaultAutoProvision)

	workspaceConfig.ServiceAccount = &controllerv1alpha1.ServiceAccountConfig{
		ServiceAccountName:   devEnvironments.ServiceAccount,
		ServiceAccountTokens: devEnvironments.ServiceAccountTokens,
		// If user's Namespace is not auto provisioned (is pre-created by admin), then ServiceAccount must be pre-created as well
		DisableCreation: pointer.Bool(!isNamespaceAutoProvisioned && devEnvironments.ServiceAccount != ""),
	}
}

func updateWorkspacePodSchedulerNameConfig(devEnvironments *chev2.CheClusterDevEnvironments, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	workspaceConfig.SchedulerName = devEnvironments.PodSchedulerName
}

func updateProjectCloneConfig(devEnvironments *chev2.CheClusterDevEnvironments, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	if devEnvironments.ProjectCloneContainer == nil {
		workspaceConfig.ProjectCloneConfig = nil
		return
	}
	if workspaceConfig.ProjectCloneConfig == nil {
		workspaceConfig.ProjectCloneConfig = &controllerv1alpha1.ProjectCloneConfig{}
	}
	container := devEnvironments.ProjectCloneContainer

	workspaceConfig.ProjectCloneConfig.Image = container.Image
	workspaceConfig.ProjectCloneConfig.ImagePullPolicy = container.ImagePullPolicy
	workspaceConfig.ProjectCloneConfig.Env = container.Env
	workspaceConfig.ProjectCloneConfig.Resources = cheResourcesToCoreV1Resources(container.Resources)
}

func disableDWOProxy(routingConfig *controllerv1alpha1.RoutingConfig) {
	// Since we create proxy configmaps to mount proxy settings, we want to disable
	// proxy handling in DWO; otherwise the env vars added by DWO will override the env
	// vars we intend to mount via configmap.
	routingConfig.ProxyConfig = &controllerv1alpha1.Proxy{}
	routingConfig.ProxyConfig.HttpProxy = pointer.String("")
	routingConfig.ProxyConfig.HttpsProxy = pointer.String("")
	routingConfig.ProxyConfig.NoProxy = pointer.String("")
}

// Returns the default container security context required for container builds.
// Returns an error if the default container security context could not be retrieved.
func getDefaultContainerSecurityContext() (*corev1.SecurityContext, error) {
	containerSecurityContext := &corev1.SecurityContext{}
	err := json.Unmarshal([]byte(defaults.GetDevEnvironmentsContainerSecurityContext()), &containerSecurityContext)
	if err != nil {
		return nil, err
	}
	return containerSecurityContext, nil
}

// cheResourcesToCoreV1Resources converts a Che resources struct to the usual Kubernetes object by directly copying fields.
// It does not set any default values or include logic for removing requests/limits that are set to "0" as it is intended
// to prepare resources for the DevWorkspace Operator, which has its own defaults and handling of "0" values.
func cheResourcesToCoreV1Resources(resources *chev2.ResourceRequirements) *corev1.ResourceRequirements {
	if resources == nil {
		return nil
	}

	result := &corev1.ResourceRequirements{}
	if resources.Limits != nil {
		result.Limits = corev1.ResourceList{}
		if resources.Limits.Memory != nil {
			result.Limits[corev1.ResourceMemory] = *resources.Limits.Memory
		}
		if resources.Limits.Cpu != nil {
			result.Limits[corev1.ResourceCPU] = *resources.Limits.Cpu
		}
	}
	if resources.Requests != nil {
		result.Requests = corev1.ResourceList{}
		if resources.Requests.Memory != nil {
			result.Requests[corev1.ResourceMemory] = *resources.Requests.Memory
		}
		if resources.Requests.Cpu != nil {
			result.Requests[corev1.ResourceCPU] = *resources.Requests.Cpu
		}
	}
	return result
}
