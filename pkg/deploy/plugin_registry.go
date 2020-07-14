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
package deploy

import (
	"encoding/json"
	"fmt"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

type PluginRegistryConfigMap struct {
	CheSidecarContainersRegistryURL          string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_URL"`
	CheSidecarContainersRegistryOrganization string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_ORGANIZATION"`
}

const (
	PluginRegistry = "plugin-registry"
)

/**
 * Create plugin registry resources unless an external registry is used.
 */
func SyncPluginRegistryToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) (bool, error) {
	pluginRegistryURL := checluster.Spec.Server.PluginRegistryUrl
	if !checluster.Spec.Server.ExternalPluginRegistry {
		var host string
		if !util.IsOpenShift {
			ingressStatus := SyncIngressToCluster(checluster, PluginRegistry, PluginRegistry, 8080, clusterAPI)
			if !util.IsTestMode() {
				if !ingressStatus.Continue {
					logrus.Infof("Waiting on ingress '%s' to be ready", PluginRegistry)
					if ingressStatus.Err != nil {
						logrus.Error(ingressStatus.Err)
					}
					return false, ingressStatus.Err
				}
			}

			if checluster.Spec.K8s.IngressStrategy == "multi-host" {
				host = PluginRegistry + "-" + checluster.Namespace + "." + checluster.Spec.K8s.IngressDomain
			} else {
				host = checluster.Spec.K8s.IngressDomain + "/" + PluginRegistry
			}
		} else {
			routeStatus := SyncRouteToCluster(checluster, PluginRegistry, PluginRegistry, 8080, clusterAPI)
			if !util.IsTestMode() {
				if !routeStatus.Continue {
					logrus.Infof("Waiting on route '%s' to be ready", PluginRegistry)
					if routeStatus.Err != nil {
						logrus.Error(routeStatus.Err)
					}

					return false, routeStatus.Err
				}
			}

			if !util.IsTestMode() {
				host = routeStatus.Route.Spec.Host
			}
		}

		if pluginRegistryURL != "" {
			if checluster.Spec.Server.TlsSupport {
				pluginRegistryURL = "https://" + host + "/v3"
			} else {
				pluginRegistryURL = "http://" + host + "/v3"
			}
		}

		if checluster.IsAirGapMode() {
			configMapData := getPluginRegistryConfigMapData(checluster)
			configMapSpec, err := GetSpecConfigMap(checluster, PluginRegistry, configMapData, clusterAPI)
			if err != nil {
				return false, err
			}

			configMap, err := SyncConfigMapToCluster(checluster, configMapSpec, clusterAPI)
			if configMap == nil {
				return false, err
			}
		}

		// Create a new registry service
		registryLabels := GetLabels(checluster, PluginRegistry)
		serviceStatus := SyncServiceToCluster(checluster, PluginRegistry, []string{"http"}, []int32{8080}, registryLabels, clusterAPI)
		if !util.IsTestMode() {
			if !serviceStatus.Continue {
				logrus.Info("Waiting on service '" + PluginRegistry + "' to be ready")
				if serviceStatus.Err != nil {
					logrus.Error(serviceStatus.Err)
				}

				return false, serviceStatus.Err
			}
		}

		// Deploy plugin registry
		deploymentStatus := SyncPluginRegistryDeploymentToCluster(checluster, clusterAPI)
		if !util.IsTestMode() {
			if !deploymentStatus.Continue {
				logrus.Info("Waiting on deployment '" + PluginRegistry + "' to be ready")
				if deploymentStatus.Err != nil {
					logrus.Error(deploymentStatus.Err)
				}

				return false, deploymentStatus.Err
			}
		}
	}

	if pluginRegistryURL != checluster.Status.PluginRegistryURL {
		checluster.Status.PluginRegistryURL = pluginRegistryURL
		if err := UpdateCheCRStatus(checluster, "status: Plugin Registry URL", pluginRegistryURL, clusterAPI); err != nil {
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
