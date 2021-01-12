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
	corev1 "k8s.io/api/core/v1"
)

func SyncPluginRegistryDeploymentToCluster(deployContext *deploy.DeployContext) deploy.DeploymentProvisioningStatus {
	registryType := "plugin"
	registryImage := util.GetValue(deployContext.CheCluster.Spec.Server.PluginRegistryImage, deploy.DefaultPluginRegistryImage(deployContext.CheCluster))
	registryImagePullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Server.PluginRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(registryImage)))
	registryMemoryLimit := util.GetValue(string(deployContext.CheCluster.Spec.Server.PluginRegistryMemoryLimit), deploy.DefaultPluginRegistryMemoryLimit)
	registryMemoryRequest := util.GetValue(string(deployContext.CheCluster.Spec.Server.PluginRegistryMemoryRequest), deploy.DefaultPluginRegistryMemoryRequest)
	probePath := "/v3/plugins/"
	pluginImagesEnv := util.GetEnvByRegExp("^.*plugin_registry_image.*$")

	clusterDeployment, err := deploy.GetClusterDeployment(deploy.PluginRegistryName, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := registry.GetSpecRegistryDeployment(
		deployContext,
		registryType,
		registryImage,
		pluginImagesEnv,
		registryImagePullPolicy,
		registryMemoryLimit,
		registryMemoryRequest,
		probePath)

	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	return deploy.SyncDeploymentToCluster(deployContext, specDeployment, clusterDeployment, nil, nil)
}
