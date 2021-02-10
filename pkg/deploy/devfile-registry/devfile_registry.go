//
// Copyright (c) 2012-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package devfile_registry

import (
	"encoding/json"
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/expose"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

type DevFileRegistryConfigMap struct {
	CheDevfileImagesRegistryURL          string `json:"CHE_DEVFILE_IMAGES_REGISTRY_URL"`
	CheDevfileImagesRegistryOrganization string `json:"CHE_DEVFILE_IMAGES_REGISTRY_ORGANIZATION"`
	CheDevfileRegistryURL                string `json:"CHE_DEVFILE_REGISTRY_URL"`
}

/**
 * Create devfile registry resources unless an external registry is used.
 */
func SyncDevfileRegistryToCluster(deployContext *deploy.DeployContext, cheHost string) (bool, error) {
	devfileRegistryURL := deployContext.CheCluster.Spec.Server.DevfileRegistryUrl
	if !deployContext.CheCluster.Spec.Server.ExternalDevfileRegistry {
		endpoint, done, err := expose.Expose(
			deployContext,
			cheHost,
			deploy.DevfileRegistryName,
			deployContext.CheCluster.Spec.Server.PluginRegistryRoute,
			deployContext.CheCluster.Spec.Server.PluginRegistryIngress,
			deploy.DevfileRegistryName)
		if !done {
			return false, err
		}

		if devfileRegistryURL == "" {
			if deployContext.CheCluster.Spec.Server.TlsSupport {
				devfileRegistryURL = "https://" + endpoint
			} else {
				devfileRegistryURL = "http://" + endpoint
			}
		}

		configMapData := getDevfileRegistryConfigMapData(deployContext.CheCluster, devfileRegistryURL)
		configMapSpec, err := deploy.GetSpecConfigMap(deployContext, deploy.DevfileRegistryName, configMapData, deploy.DevfileRegistryName)
		if err != nil {
			return false, err
		}

		configMap, err := deploy.SyncConfigMapToCluster(deployContext, configMapSpec)
		if configMap == nil {
			return false, err
		}

		// Create a new registry service
		serviceStatus := deploy.SyncServiceToCluster(deployContext, deploy.DevfileRegistryName, []string{"http"}, []int32{8080}, deploy.DevfileRegistryName)
		if !util.IsTestMode() {
			if !serviceStatus.Continue {
				logrus.Info("Waiting on service '" + deploy.DevfileRegistryName + "' to be ready")
				if serviceStatus.Err != nil {
					logrus.Error(serviceStatus.Err)
				}

				return false, serviceStatus.Err
			}
		}

		deployContext.InternalService.DevfileRegistryHost = fmt.Sprintf("http://%s.%s.svc:8080", deploy.DevfileRegistryName, deployContext.CheCluster.Namespace)

		// Deploy devfile registry
		provisioned, err := SyncDevfileRegistryDeploymentToCluster(deployContext)
		if !util.IsTestMode() {
			if !provisioned {
				logrus.Info("Waiting on deployment '" + deploy.DevfileRegistryName + "' to be ready")
				if err != nil {
					logrus.Error(err)
				}
				return provisioned, err
			}
		}
	}

	if devfileRegistryURL != deployContext.CheCluster.Status.DevfileRegistryURL {
		deployContext.CheCluster.Status.DevfileRegistryURL = devfileRegistryURL
		if err := deploy.UpdateCheCRStatus(deployContext, "status: Devfile Registry URL", devfileRegistryURL); err != nil {
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
