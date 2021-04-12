//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package pluginregistry

import (
	"encoding/json"
)

type PluginRegistryConfigMap struct {
	CheSidecarContainersRegistryURL          string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_URL"`
	CheSidecarContainersRegistryOrganization string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_ORGANIZATION"`
}

func (p *PluginRegistry) GetConfigMapData() (map[string]string, error) {
	pluginRegistryEnv := make(map[string]string)
	data := &PluginRegistryConfigMap{
		CheSidecarContainersRegistryURL:          p.deployContext.CheCluster.Spec.Server.AirGapContainerRegistryHostname,
		CheSidecarContainersRegistryOrganization: p.deployContext.CheCluster.Spec.Server.AirGapContainerRegistryOrganization,
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(out, &pluginRegistryEnv)
	if err != nil {
		return nil, err
	}

	return pluginRegistryEnv, nil
}
