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
	"encoding/json"
	"fmt"

	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/expose"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

type PluginRegistryConfigMap struct {
	CheSidecarContainersRegistryURL          string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_URL"`
	CheSidecarContainersRegistryOrganization string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_ORGANIZATION"`
}

/**
 * Create plugin registry resources unless an external registry is used.
 */
func SyncPluginRegistryToCluster(deployContext *deploy.DeployContext, cheHost string) (bool, error) {
	pluginRegistryURL := deployContext.CheCluster.Spec.Server.PluginRegistryUrl
	if !deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		additionalLabels := (map[bool]string{true: deployContext.CheCluster.Spec.Server.PluginRegistryRoute.Labels, false: deployContext.CheCluster.Spec.Server.PluginRegistryIngress.Labels})[util.IsOpenShift]
		endpoint, done, err := expose.Expose(deployContext, cheHost, deploy.PluginRegistryName, additionalLabels, deploy.PluginRegistryName)
		if !done {
			return false, err
		}

		if pluginRegistryURL == "" {
			if deployContext.CheCluster.Spec.Server.TlsSupport {
				pluginRegistryURL = "https://" + endpoint + "/v3"
			} else {
				pluginRegistryURL = "http://" + endpoint + "/v3"
			}
		}

		if deployContext.CheCluster.IsAirGapMode() {
			configMapData := getPluginRegistryConfigMapData(deployContext.CheCluster)
			configMapSpec, err := deploy.GetSpecConfigMap(deployContext, deploy.PluginRegistryName, configMapData, deploy.PluginRegistryName)
			if err != nil {
				return false, err
			}

			configMap, err := deploy.SyncConfigMapToCluster(deployContext, configMapSpec)
			if configMap == nil {
				return false, err
			}
		}

		// Create a new registry service
		serviceStatus := deploy.SyncServiceToCluster(deployContext, deploy.PluginRegistryName, []string{"http"}, []int32{8080}, deploy.PluginRegistryName)
		if !util.IsTestMode() {
			if !serviceStatus.Continue {
				logrus.Info("Waiting on service '" + deploy.PluginRegistryName + "' to be ready")
				if serviceStatus.Err != nil {
					logrus.Error(serviceStatus.Err)
				}

				return false, serviceStatus.Err
			}
		}

		deployContext.InternalService.PluginRegistryHost = fmt.Sprintf("http://%s.%s.svc:8080/v3", deploy.PluginRegistryName, deployContext.CheCluster.Namespace)

		// Deploy plugin registry
		deploymentStatus := SyncPluginRegistryDeploymentToCluster(deployContext)
		if !util.IsTestMode() {
			if !deploymentStatus.Continue {
				logrus.Info("Waiting on deployment '" + deploy.PluginRegistryName + "' to be ready")
				if deploymentStatus.Err != nil {
					logrus.Error(deploymentStatus.Err)
				}

				return false, deploymentStatus.Err
			}
		}
	}

	if pluginRegistryURL != deployContext.CheCluster.Status.PluginRegistryURL {
		deployContext.CheCluster.Status.PluginRegistryURL = pluginRegistryURL
		if err := deploy.UpdateCheCRStatus(deployContext, "status: Plugin Registry URL", pluginRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func getPluginRegistryConfigMapData(cr *orgv1.CheCluster) map[string]string {
	pluginRegistryEnv := make(map[string]string)
	data := &PluginRegistryConfigMap{
		CheSidecarContainersRegistryURL:          cr.Spec.Server.AirGapContainerRegistryHostname,
		CheSidecarContainersRegistryOrganization: cr.Spec.Server.AirGapContainerRegistryOrganization,
	}

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}

	err = json.Unmarshal(out, &pluginRegistryEnv)
	if err != nil {
		fmt.Println(err)
	}

	return pluginRegistryEnv
}
