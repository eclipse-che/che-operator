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
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
)

type PluginRegistryConfigMap struct {
	CheSidecarContainersRegistryURL          string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_URL"`
	CheSidecarContainersRegistryOrganization string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_ORGANIZATION"`
}

/**
 * Create plugin registry resources unless an external registry is used.
 */
func SyncPluginRegistryToCluster(deployContext *deploy.DeployContext) (bool, error) {
	pluginRegistryURL := deployContext.CheCluster.Spec.Server.PluginRegistryUrl
	if !deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		endpoint, done, err := expose.Expose(
			deployContext,
			deploy.PluginRegistryName,
			deployContext.CheCluster.Spec.Server.PluginRegistryRoute,
			deployContext.CheCluster.Spec.Server.PluginRegistryIngress)
		if !done {
			return false, err
		}

		if pluginRegistryURL == "" {
			if deployContext.CheCluster.Spec.Server.TlsSupport {
				pluginRegistryURL = "https://" + endpoint
			} else {
				pluginRegistryURL = "http://" + endpoint
			}
			// append the API version to plugin registry
			if !strings.HasSuffix(pluginRegistryURL, "/") {
				pluginRegistryURL = pluginRegistryURL + "/v3"
			} else {
				pluginRegistryURL = pluginRegistryURL + "v3"
			}
		}

		if deployContext.CheCluster.IsAirGapMode() {
			configMapData := getPluginRegistryConfigMapData(deployContext.CheCluster)
			done, err := deploy.SyncConfigMapDataToCluster(deployContext, deploy.PluginRegistryName, configMapData, deploy.PluginRegistryName)
			if !done {
				return false, err
			}
		}

		// Create a new registry service
		done, err = deploy.SyncServiceToCluster(deployContext, deploy.PluginRegistryName, []string{"http"}, []int32{8080}, deploy.PluginRegistryName)
		if !done {
			if err != nil {
				logrus.Error(err)
			}
			return false, err
		}

		deployContext.InternalService.PluginRegistryHost = fmt.Sprintf("http://%s.%s.svc:8080/v3", deploy.PluginRegistryName, deployContext.CheCluster.Namespace)

		// Deploy plugin registry
		provisioned, err := SyncPluginRegistryDeploymentToCluster(deployContext)
		if !util.IsTestMode() {
			if !provisioned {
				logrus.Info("Waiting on deployment '" + deploy.PluginRegistryName + "' to be ready")
				if err != nil {
					logrus.Error(err)
				}
				return provisioned, err
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
