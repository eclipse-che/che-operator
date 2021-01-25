//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package plugin_registry

import (
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/registry"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func SyncPluginRegistryDeploymentToCluster(deployContext *deploy.DeployContext) (bool, error) {
	clusterDeployment, err := deploy.GetClusterDeployment(deploy.PluginRegistryName, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return false, err
	}

	specDeployment, err := GetPluginRegistrySpecDeployment(deployContext)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentToCluster(deployContext, specDeployment, clusterDeployment, nil, nil)
}

func GetPluginRegistrySpecDeployment(deployContext *deploy.DeployContext) (*appsv1.Deployment, error) {
	registryType := "plugin"
	registryImage := util.GetValue(deployContext.CheCluster.Spec.Server.PluginRegistryImage, deploy.DefaultPluginRegistryImage(deployContext.CheCluster))
	registryImagePullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Server.PluginRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(registryImage)))
	probePath := "/v3/plugins/"
	pluginImagesEnv := util.GetEnvByRegExp("^.*plugin_registry_image.*$")

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: util.GetResourceQuantity(
				deployContext.CheCluster.Spec.Server.PluginRegistryMemoryRequest,
				deploy.DefaultPluginRegistryMemoryRequest),
			corev1.ResourceCPU: util.GetResourceQuantity(
				deployContext.CheCluster.Spec.Server.PluginRegistryCpuRequest,
				deploy.DefaultPluginRegistryCpuRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: util.GetResourceQuantity(
				deployContext.CheCluster.Spec.Server.PluginRegistryMemoryLimit,
				deploy.DefaultPluginRegistryMemoryLimit),
			corev1.ResourceCPU: util.GetResourceQuantity(
				deployContext.CheCluster.Spec.Server.PluginRegistryCpuLimit,
				deploy.DefaultPluginRegistryCpuLimit),
		},
	}

	specDeployment, err := registry.GetSpecRegistryDeployment(
		deployContext,
		registryType,
		registryImage,
		pluginImagesEnv,
		registryImagePullPolicy,
		resources,
		probePath)
	if err != nil {
		return nil, err
	}

	return specDeployment, nil
}
