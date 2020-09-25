package server

import (
	"github.com/eclipse/che-operator/pkg/deploy"
	"k8s.io/api/core/v1"
)

func GetSpecCheService(deployContext *deploy.DeployContext) (*v1.Service, error) {
	portName := []string{"http"}
	portNumber := []int32{8080}
	labels := deploy.GetLabels(deployContext.CheCluster, deploy.DefaultCheFlavor(deployContext.CheCluster))

	if deployContext.CheCluster.Spec.Metrics.Enable {
		portName = append(portName, "metrics")
		portNumber = append(portNumber, deploy.DefaultCheMetricsPort)
	}

	if deployContext.CheCluster.Spec.Server.CheDebug == "true" {
		portName = append(portName, "debug")
		portNumber = append(portNumber, deploy.DefaultCheDebugPort)
	}

	return deploy.GetSpecService(deployContext, deploy.CheServiceName, portName, portNumber, labels)
}

func SyncCheServiceToCluster(deployContext *deploy.DeployContext) deploy.ServiceProvisioningStatus {
	specService, err := GetSpecCheService(deployContext)
	if err != nil {
		return deploy.ServiceProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	return deploy.DoSyncServiceToCluster(deployContext, specService)
}
