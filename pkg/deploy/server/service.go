//
// Copyright (c) 2020-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package server

import (
	"github.com/eclipse/che-operator/pkg/deploy"
	v1 "k8s.io/api/core/v1"
)

func GetSpecCheService(deployContext *deploy.DeployContext) (*v1.Service, error) {
	portName := []string{"http"}
	portNumber := []int32{8080}

	if deployContext.CheCluster.Spec.Metrics.Enable {
		portName = append(portName, "metrics")
		portNumber = append(portNumber, deploy.DefaultCheMetricsPort)
	}

	if deployContext.CheCluster.Spec.Server.CheDebug == "true" {
		portName = append(portName, "debug")
		portNumber = append(portNumber, deploy.DefaultCheDebugPort)
	}

	return deploy.GetSpecService(deployContext, deploy.CheServiceName, portName, portNumber, deploy.DefaultCheFlavor(deployContext.CheCluster))
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
