//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package devfileregistry

import (
	"encoding/json"
)

type DevFileRegistryConfigMap struct {
	CheDevfileImagesRegistryURL          string `json:"CHE_DEVFILE_IMAGES_REGISTRY_URL"`
	CheDevfileImagesRegistryOrganization string `json:"CHE_DEVFILE_IMAGES_REGISTRY_ORGANIZATION"`
	CheDevfileRegistryURL                string `json:"CHE_DEVFILE_REGISTRY_URL"`
}

func (p *DevfileRegistry) GetConfigMapData() (map[string]string, error) {
	devfileRegistryEnv := make(map[string]string)
	data := &DevFileRegistryConfigMap{
		CheDevfileImagesRegistryURL:          p.deployContext.CheCluster.Spec.Server.AirGapContainerRegistryHostname,
		CheDevfileImagesRegistryOrganization: p.deployContext.CheCluster.Spec.Server.AirGapContainerRegistryOrganization,
		CheDevfileRegistryURL:                p.deployContext.CheCluster.Status.DevfileRegistryURL,
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(out, &devfileRegistryEnv)
	if err != nil {
		return nil, err
	}

	return devfileRegistryEnv, nil
}
