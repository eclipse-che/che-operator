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
	PluginRegistry              = "plugin-registry"
	pluginRegistryGatewayConfig = "che-gateway-route-plugin-registry"
)

/**
 * Create plugin registry resources unless an external registry is used.
 */
func SyncPluginRegistryToCluster(deployContext *DeployContext, cheHost string) (bool, error) {
	pluginRegistryURL := deployContext.CheCluster.Spec.Server.PluginRegistryUrl
	if !deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		var host string
		exposureStrategy := util.GetServerExposureStrategy(deployContext.CheCluster, DefaultServerExposureStrategy)
		singleHostExposureType := GetSingleHostExposureType(deployContext.CheCluster)
		useGateway := exposureStrategy == "single-host" && (util.IsOpenShift || singleHostExposureType == "gateway")

		if !util.IsOpenShift {
			if useGateway {
				cfg := GetGatewayRouteConfig(deployContext.CheCluster, pluginRegistryGatewayConfig, "/"+PluginRegistry, 10, "http://"+PluginRegistry+":8080", true)
				clusterCfg, err := SyncConfigMapToCluster(deployContext, &cfg)
				if !util.IsTestMode() {
					if clusterCfg == nil {
						if err != nil {
							logrus.Error(err)
						}
						return false, err
					}
				}
				if err := DeleteIngressIfExists(PluginRegistry, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
			} else {
				ingress, err := SyncIngressToCluster(deployContext, PluginRegistry, "", PluginRegistry, 8080)
				if !util.IsTestMode() {
					if ingress == nil {
						logrus.Infof("Waiting on ingress '%s' to be ready", PluginRegistry)
						if err != nil {
							logrus.Error(err)
						}
						return false, err
					}
				}
				if err := DeleteGatewayRouteConfig(pluginRegistryGatewayConfig, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
			}

			if exposureStrategy == "multi-host" {
				host = PluginRegistry + "-" + deployContext.CheCluster.Namespace + "." + deployContext.CheCluster.Spec.K8s.IngressDomain
			} else {
				host = cheHost + "/" + PluginRegistry
			}
		} else {
			if useGateway {
				cfg := GetGatewayRouteConfig(deployContext.CheCluster, pluginRegistryGatewayConfig, "/"+PluginRegistry, 10, "http://"+PluginRegistry+":8080", true)
				clusterCfg, err := SyncConfigMapToCluster(deployContext, &cfg)
				if !util.IsTestMode() {
					if clusterCfg == nil {
						if err != nil {
							logrus.Error(err)
						}
						return false, err
					}
				}
				if err := DeleteRouteIfExists(PluginRegistry, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}

				host = cheHost + "/" + PluginRegistry
			} else {
				route, err := SyncRouteToCluster(deployContext, PluginRegistry, "", PluginRegistry, 8080)
				if !util.IsTestMode() {
					if route == nil {
						logrus.Infof("Waiting on route '%s' to be ready", PluginRegistry)
						if err != nil {
							logrus.Error(err)
						}

						return false, err
					}
				}
				if err := DeleteGatewayRouteConfig(pluginRegistryGatewayConfig, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}

				if !util.IsTestMode() {
					host = route.Spec.Host
				}
			}
		}

		if pluginRegistryURL == "" {
			if deployContext.CheCluster.Spec.Server.TlsSupport {
				pluginRegistryURL = "https://" + host + "/v3"
			} else {
				pluginRegistryURL = "http://" + host + "/v3"
			}
		}

		if deployContext.CheCluster.IsAirGapMode() {
			configMapData := getPluginRegistryConfigMapData(deployContext.CheCluster)
			configMapSpec, err := GetSpecConfigMap(deployContext, PluginRegistry, configMapData)
			if err != nil {
				return false, err
			}

			configMap, err := SyncConfigMapToCluster(deployContext, configMapSpec)
			if configMap == nil {
				return false, err
			}
		}

		// Create a new registry service
		registryLabels := GetLabels(deployContext.CheCluster, PluginRegistry)
		serviceStatus := SyncServiceToCluster(deployContext, PluginRegistry, []string{"http"}, []int32{8080}, registryLabels)
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
		deploymentStatus := SyncPluginRegistryDeploymentToCluster(deployContext)
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

	if pluginRegistryURL != deployContext.CheCluster.Status.PluginRegistryURL {
		deployContext.CheCluster.Status.PluginRegistryURL = pluginRegistryURL
		if err := UpdateCheCRStatus(deployContext, "status: Plugin Registry URL", pluginRegistryURL); err != nil {
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
