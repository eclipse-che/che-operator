//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"fmt"
	"strconv"

	"github.com/eclipse-che/che-operator/pkg/common/constants"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
)

type PluginRegistryConfigMap struct {
	CheSidecarContainersRegistryURL          string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_URL"`
	CheSidecarContainersRegistryOrganization string `json:"CHE_SIDECAR_CONTAINERS_REGISTRY_ORGANIZATION"`
	ChePluginRegistryURL                     string `json:"CHE_PLUGIN_REGISTRY_URL"`
	ChePluginRegistryInternalURL             string `json:"CHE_PLUGIN_REGISTRY_INTERNAL_URL"`
	StartOpenVSX                             string `json:"START_OPENVSX"`
}

func (p *PluginRegistryReconciler) getConfigMapData(cheCtx *chetypes.CheContext) (map[string]string, error) {
	pluginRegistryEnv := make(map[string]string)
	data := &PluginRegistryConfigMap{
		CheSidecarContainersRegistryURL:          cheCtx.CheCluster.Spec.ContainerRegistry.Hostname,
		CheSidecarContainersRegistryOrganization: cheCtx.CheCluster.Spec.ContainerRegistry.Organization,
		ChePluginRegistryURL:                     cheCtx.CheCluster.Status.PluginRegistryURL,
		ChePluginRegistryInternalURL:             fmt.Sprintf("http://%s.%s.svc:8080", constants.PluginRegistryName, cheCtx.CheCluster.Namespace),
		StartOpenVSX:                             strconv.FormatBool(cheCtx.CheCluster.IsInternalPluginRegistryWithOpenVSXEnabled()),
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
