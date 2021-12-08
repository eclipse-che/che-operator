//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/registry"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func (p *PluginRegistryReconciler) getPluginRegistryDeploymentSpec(ctx *deploy.DeployContext) *appsv1.Deployment {
	registryType := "plugin"
	registryImage := util.GetValue(ctx.CheCluster.Spec.Server.PluginRegistryImage, deploy.DefaultPluginRegistryImage(ctx.CheCluster))
	registryImagePullPolicy := corev1.PullPolicy(util.GetValue(string(ctx.CheCluster.Spec.Server.PluginRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(registryImage)))
	probePath := "/v3/plugins/"
	pluginImagesEnv := util.GetEnvByRegExp("^.*plugin_registry_image.*$")

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.PluginRegistryMemoryRequest,
				deploy.DefaultPluginRegistryMemoryRequest),
			corev1.ResourceCPU: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.PluginRegistryCpuRequest,
				deploy.DefaultPluginRegistryCpuRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.PluginRegistryMemoryLimit,
				deploy.DefaultPluginRegistryMemoryLimit),
			corev1.ResourceCPU: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.PluginRegistryCpuLimit,
				deploy.DefaultPluginRegistryCpuLimit),
		},
	}

	return registry.GetSpecRegistryDeployment(
		ctx,
		registryType,
		registryImage,
		pluginImagesEnv,
		registryImagePullPolicy,
		resources,
		probePath)
}
