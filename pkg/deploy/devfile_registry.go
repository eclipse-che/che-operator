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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

type DevFileRegistryConfigMap struct {
	CheDevfileImagesRegistryURL          string `json:"CHE_DEVFILE_IMAGES_REGISTRY_URL"`
	CheDevfileImagesRegistryOrganization string `json:"CHE_DEVFILE_IMAGES_REGISTRY_ORGANIZATION"`
	CheDevfileRegistryURL                string `json:"CHE_DEVFILE_REGISTRY_URL"`
}

const (
	DevfileRegistry              = "devfile-registry"
	devfileRegistryGatewayConfig = "che-gateway-route-devfile-registry"
)

/**
 * Create devfile registry resources unless an external registry is used.
 */
func SyncDevfileRegistryToCluster(deployContext *DeployContext, cheHost string) (bool, error) {
	devfileRegistryURL := deployContext.CheCluster.Spec.Server.DevfileRegistryUrl
	if !deployContext.CheCluster.Spec.Server.ExternalDevfileRegistry {
		var endpoint string
		var domain string
		exposureStrategy := util.GetServerExposureStrategy(deployContext.CheCluster, DefaultServerExposureStrategy)
		singleHostExposureType := GetSingleHostExposureType(deployContext.CheCluster)
		useGateway := exposureStrategy == "single-host" && (util.IsOpenShift || singleHostExposureType == "gateway")
		if exposureStrategy == "multi-host" {
			// this won't get used on openshift, because there we're intentionally let Openshift decide on the domain name
			domain = DevfileRegistry + "-" + deployContext.CheCluster.Namespace + "." + deployContext.CheCluster.Spec.K8s.IngressDomain
			endpoint = domain
		} else {
			domain = cheHost
			endpoint = domain + "/" + DevfileRegistry
		}
		if !util.IsOpenShift {

			if useGateway {
				cfg := GetGatewayRouteConfig(deployContext, devfileRegistryGatewayConfig, "/"+DevfileRegistry, 10, "http://"+DevfileRegistry+":8080", true)
				clusterCfg, err := SyncConfigMapToCluster(deployContext, &cfg)
				if !util.IsTestMode() {
					if clusterCfg == nil {
						if err != nil {
							logrus.Error(err)
						}
						return false, err
					}
				}
				if err := DeleteIngressIfExists(DevfileRegistry, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
			} else {
				additionalLabels := deployContext.CheCluster.Spec.DevfileRegistry.Ingress.Labels
				ingress, err := SyncIngressToCluster(deployContext, DevfileRegistry, domain, DevfileRegistry, 8080, additionalLabels)
				if !util.IsTestMode() {
					if ingress == nil {
						logrus.Infof("Waiting on ingress '%s' to be ready", DevfileRegistry)
						if err != nil {
							logrus.Error(err)
						}
						return false, err
					}
				}
				if err := DeleteGatewayRouteConfig(devfileRegistryGatewayConfig, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
			}
		} else {
			if useGateway {
				cfg := GetGatewayRouteConfig(deployContext, devfileRegistryGatewayConfig, "/"+DevfileRegistry, 10, "http://"+DevfileRegistry+":8080", true)
				clusterCfg, err := SyncConfigMapToCluster(deployContext, &cfg)
				if !util.IsTestMode() {
					if clusterCfg == nil {
						if err != nil {
							logrus.Error(err)
						}
						return false, err
					}
				}
				if err := DeleteRouteIfExists(DevfileRegistry, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
			} else {
				// the empty string for a host is intentional here - we let OpenShift decide on the hostname
				additionalLabels := deployContext.CheCluster.Spec.DevfileRegistry.Route.Labels
				route, err := SyncRouteToCluster(deployContext, DevfileRegistry, "", DevfileRegistry, 8080, additionalLabels)
				if !util.IsTestMode() {
					if route == nil {
						logrus.Infof("Waiting on route '%s' to be ready", DevfileRegistry)
						if err != nil {
							logrus.Error(err)
						}

						return false, err
					}
				}
				if err := DeleteGatewayRouteConfig(devfileRegistryGatewayConfig, deployContext); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
				if !util.IsTestMode() {
					endpoint = route.Spec.Host
				}
			}
		}

		if devfileRegistryURL == "" {
			if deployContext.CheCluster.Spec.Server.TlsSupport {
				devfileRegistryURL = "https://" + endpoint
			} else {
				devfileRegistryURL = "http://" + endpoint
			}
		}

		configMapData := getDevfileRegistryConfigMapData(deployContext.CheCluster, devfileRegistryURL)
		configMapSpec, err := GetSpecConfigMap(deployContext, DevfileRegistry, configMapData)
		if err != nil {
			return false, err
		}

		configMap, err := SyncConfigMapToCluster(deployContext, configMapSpec)
		if configMap == nil {
			return false, err
		}

		// Create a new registry service
		registryLabels := GetLabels(deployContext.CheCluster, DevfileRegistry)
		serviceStatus := SyncServiceToCluster(deployContext, DevfileRegistry, []string{"http"}, []int32{8080}, registryLabels)
		if !util.IsTestMode() {
			if !serviceStatus.Continue {
				logrus.Info("Waiting on service '" + DevfileRegistry + "' to be ready")
				if serviceStatus.Err != nil {
					logrus.Error(serviceStatus.Err)
				}

				return false, serviceStatus.Err
			}
		}

		// Deploy devfile registry
		deploymentStatus := SyncDevfileRegistryDeploymentToCluster(deployContext)
		if !util.IsTestMode() {
			if !deploymentStatus.Continue {
				logrus.Info("Waiting on deployment '" + DevfileRegistry + "' to be ready")
				if deploymentStatus.Err != nil {
					logrus.Error(deploymentStatus.Err)
				}

				return false, deploymentStatus.Err
			}
		}
	}

	if devfileRegistryURL != deployContext.CheCluster.Status.DevfileRegistryURL {
		deployContext.CheCluster.Status.DevfileRegistryURL = devfileRegistryURL
		if err := UpdateCheCRStatus(deployContext, "status: Devfile Registry URL", devfileRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func getDevfileRegistryConfigMapData(cr *orgv1.CheCluster, endpoint string) map[string]string {
	devfileRegistryEnv := make(map[string]string)
	data := &DevFileRegistryConfigMap{
		CheDevfileImagesRegistryURL:          cr.Spec.Server.AirGapContainerRegistryHostname,
		CheDevfileImagesRegistryOrganization: cr.Spec.Server.AirGapContainerRegistryOrganization,
		CheDevfileRegistryURL:                endpoint,
	}

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}

	err = json.Unmarshal(out, &devfileRegistryEnv)
	if err != nil {
		fmt.Println(err)
	}
	return devfileRegistryEnv
}
