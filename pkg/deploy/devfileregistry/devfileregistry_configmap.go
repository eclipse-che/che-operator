//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/constants"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
)

type DevFileRegistryConfigMap struct {
	CheDevfileImagesRegistryURL          string `json:"CHE_DEVFILE_IMAGES_REGISTRY_URL"`
	CheDevfileImagesRegistryOrganization string `json:"CHE_DEVFILE_IMAGES_REGISTRY_ORGANIZATION"`
	CheDevfileRegistryURL                string `json:"CHE_DEVFILE_REGISTRY_URL"`
	CheDevfileRegistryInternalURL        string `json:"CHE_DEVFILE_REGISTRY_INTERNAL_URL"`
}

func (d *DevfileRegistryReconciler) getConfigMapData(ctx *chetypes.DeployContext) (map[string]string, error) {
	devfileRegistryEnv := make(map[string]string)
	data := &DevFileRegistryConfigMap{
		CheDevfileImagesRegistryURL:          ctx.CheCluster.Spec.ContainerRegistry.Hostname,
		CheDevfileImagesRegistryOrganization: ctx.CheCluster.Spec.ContainerRegistry.Organization,
		CheDevfileRegistryURL:                ctx.CheCluster.Status.DevfileRegistryURL,
		CheDevfileRegistryInternalURL:        fmt.Sprintf("http://%s.%s.svc:8080", constants.DevfileRegistryName, ctx.CheCluster.Namespace),
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
