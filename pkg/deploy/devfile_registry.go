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
	DevfileRegistry = "devfile-registry"
)

/**
 * Create devfile registry resources unless an external registry is used.
 */
func SyncDevfileRegistryToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) (bool, error) {
	devfileRegistryURL := checluster.Spec.Server.DevfileRegistryUrl
	if !checluster.Spec.Server.ExternalDevfileRegistry {
		var host string
		if !util.IsOpenShift {
			ingressStatus := SyncIngressToCluster(checluster, DevfileRegistry, DevfileRegistry, 8080, clusterAPI)
			if !util.IsTestMode() {
				if !ingressStatus.Continue {
					logrus.Infof("Waiting on ingress '%s' to be ready", DevfileRegistry)
					if ingressStatus.Err != nil {
						logrus.Error(ingressStatus.Err)
					}
					return false, ingressStatus.Err
				}
			}

			ingressStrategy := util.GetValue(checluster.Spec.K8s.IngressStrategy, DefaultIngressStrategy)
			if ingressStrategy == "multi-host" {
				host = DevfileRegistry + "-" + checluster.Namespace + "." + checluster.Spec.K8s.IngressDomain
			} else {
				host = checluster.Spec.K8s.IngressDomain + "/" + DevfileRegistry
			}
		} else {
			routeStatus := SyncRouteToCluster(checluster, DevfileRegistry, DevfileRegistry, 8080, clusterAPI)
			if !util.IsTestMode() {
				if !routeStatus.Continue {
					logrus.Infof("Waiting on route '%s' to be ready", DevfileRegistry)
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

		if devfileRegistryURL == "" {
			if checluster.Spec.Server.TlsSupport {
				devfileRegistryURL = "https://" + host
			} else {
				devfileRegistryURL = "http://" + host
			}
		}

		configMapData := getDevfileRegistryConfigMapData(checluster, devfileRegistryURL)
		configMapSpec, err := GetSpecConfigMap(checluster, DevfileRegistry, configMapData, clusterAPI)
		if err != nil {
			return false, err
		}

		configMap, err := SyncConfigMapToCluster(checluster, configMapSpec, clusterAPI)
		if configMap == nil {
			return false, err
		}

		// Create a new registry service
		registryLabels := GetLabels(checluster, DevfileRegistry)
		serviceStatus := SyncServiceToCluster(checluster, DevfileRegistry, []string{"http"}, []int32{8080}, registryLabels, clusterAPI)
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
		deploymentStatus := SyncDevfileRegistryDeploymentToCluster(checluster, clusterAPI)
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

	if devfileRegistryURL != checluster.Status.DevfileRegistryURL {
		checluster.Status.DevfileRegistryURL = devfileRegistryURL
		if err := UpdateCheCRStatus(checluster, "status: Devfile Registry URL", devfileRegistryURL, clusterAPI); err != nil {
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
