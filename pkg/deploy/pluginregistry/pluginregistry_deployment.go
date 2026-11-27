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

package pluginregistry

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy/registry"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func (p *PluginRegistryReconciler) getPluginRegistryDeploymentSpec(cheCtx *chetypes.CheContext) (*appsv1.Deployment, error) {
	registryType := "plugin"
	registryImage := defaults.GetPluginRegistryImage(cheCtx.CheCluster)
	registryImagePullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(registryImage))
	probePath := "/openvsx/api/version"
	pluginImagesEnv := utils.GetGetArchitectureDependentEnvsByRegExp("^.*plugin_registry_image.*$")

	resRequirements := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(constants.DefaultPluginRegistryMemoryRequest),
			corev1.ResourceCPU:    resource.MustParse(constants.DefaultPluginRegistryCpuRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(constants.DefaultPluginRegistryMemoryLimit),
			corev1.ResourceCPU:    resource.MustParse(constants.DefaultPluginRegistryCpuLimit),
		},
	}

	if cheCtx.CheCluster.IsInternalPluginRegistryWithOpenVSXEnabled() {
		resRequirements.Requests[corev1.ResourceMemory] = resource.MustParse(constants.DefaultPluginRegistryMemoryRequestEmbeddedOpenVSXRegistry)
		resRequirements.Limits[corev1.ResourceMemory] = resource.MustParse(constants.DefaultPluginRegistryMemoryLimitEmbeddedOpenVSXRegistry)
	}

	deployment := registry.GetSpecRegistryDeployment(
		cheCtx,
		registryType,
		registryImage,
		pluginImagesEnv,
		registryImagePullPolicy,
		resRequirements,
		probePath)

	if cheCtx.CheCluster.IsInternalPluginRegistryWithOpenVSXEnabled() {
		// Add time to start embedded VSX registry
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 300
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = 30
	}

	if err := resources.Deployment.TrimCPULimitIfNoLimitRange(cheCtx, deployment); err != nil {
		return nil, err
	}

	pod := &deployment.Spec.Template.Spec
	resources.Pod.EnsureSecurityStandards(pod, constants.DefaultSecurityContextRunAsUser, constants.DefaultSecurityContextFsGroup)

	overrides := cheCtx.CheCluster.Spec.Components.PluginRegistry.Deployment
	resources.Deployment.ApplyOverrides(deployment, overrides)

	return deployment, nil
}
