package devfile_registry

import (
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/registry"
	"github.com/eclipse/che-operator/pkg/util"
	"k8s.io/api/core/v1"
)

func SyncDevfileRegistryDeploymentToCluster(deployContext *deploy.DeployContext) deploy.DeploymentProvisioningStatus {
	registryType := "devfile"
	registryImage := util.GetValue(deployContext.CheCluster.Spec.Server.DevfileRegistryImage, deploy.DefaultDevfileRegistryImage(deployContext.CheCluster))
	registryImagePullPolicy := v1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Server.PluginRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(registryImage)))
	registryMemoryLimit := util.GetValue(string(deployContext.CheCluster.Spec.Server.DevfileRegistryMemoryLimit), deploy.DefaultDevfileRegistryMemoryLimit)
	registryMemoryRequest := util.GetValue(string(deployContext.CheCluster.Spec.Server.DevfileRegistryMemoryRequest), deploy.DefaultDevfileRegistryMemoryRequest)
	probePath := "/devfiles/"
	devfileImagesEnv := util.GetEnvByRegExp("^.*devfile_registry_image.*$")

	clusterDeployment, err := deploy.GetClusterDeployment(deploy.DevfileRegistry, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := registry.GetSpecRegistryDeployment(
		deployContext,
		registryType,
		registryImage,
		devfileImagesEnv,
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
