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
package devfileregistry

import (
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
)

type DevfileRegistry struct {
	devfileRegistryUrl string
	deployContext      *deploy.DeployContext
}

func NewDevfileRegistry(deployContext *deploy.DeployContext) *DevfileRegistry {
	return &DevfileRegistry{
		deployContext: deployContext,
	}
}

func (p *DevfileRegistry) SyncAll() (bool, error) {
	done, err := p.SyncService()
	if !done {
		return false, err
	}

	done, err = p.ExposeEndpoint()
	if !done {
		return false, err
	}

	done, err = p.SyncConfigMap()
	if !done {
		return false, err
	}

	done, err = p.SyncDeployment()
	if !done {
		return false, err
	}

	return true, nil
}

func (p *DevfileRegistry) SyncService() (bool, error) {
	return deploy.SyncServiceToCluster(
		p.deployContext,
		deploy.DevfileRegistryName,
		[]string{"http"},
		[]int32{8080},
		deploy.DevfileRegistryName)
}

func (p *DevfileRegistry) SyncConfigMap() (bool, error) {
	data, err := p.GetConfigMapData()
	if err != nil {
		return false, err
	}
	return deploy.SyncConfigMapDataToCluster(p.deployContext, deploy.DevfileRegistryName, data, deploy.DevfileRegistryName)
}

func (p *DevfileRegistry) ExposeEndpoint() (bool, error) {
	endpoint, done, err := expose.Expose(
		p.deployContext,
		deploy.DevfileRegistryName,
		p.deployContext.CheCluster.Spec.Server.DevfileRegistryRoute,
		p.deployContext.CheCluster.Spec.Server.DevfileRegistryIngress)

	if done {
		p.devfileRegistryUrl = computeDevfileRegistryUrl(endpoint, p.deployContext.CheCluster)
	} else {
		p.devfileRegistryUrl = ""
	}
	return done, err
}

func (p *DevfileRegistry) UpdateStatus() (bool, error) {
	devfileRegistryUrl := p.devfileRegistryUrl

	// `Spec.Server.DevfileRegistryUrl` is deprecated in favor of `Server.ExternalDevfileRegistries`
	if p.deployContext.CheCluster.Spec.Server.DevfileRegistryUrl != "" {
		devfileRegistryUrl += " " + p.deployContext.CheCluster.Spec.Server.DevfileRegistryUrl
	}

	for _, r := range p.deployContext.CheCluster.Spec.Server.ExternalDevfileRegistries {
		if strings.Index(devfileRegistryUrl, r.Url) == -1 {
			devfileRegistryUrl += " " + r.Url
		}
	}
	devfileRegistryUrl = strings.TrimSpace(devfileRegistryUrl)

	if devfileRegistryUrl != p.deployContext.CheCluster.Status.DevfileRegistryURL {
		p.deployContext.CheCluster.Status.DevfileRegistryURL = devfileRegistryUrl
		if err := deploy.UpdateCheCRStatus(p.deployContext, "status: Devfile Registry URL(s)", devfileRegistryUrl); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *DevfileRegistry) SyncDeployment() (bool, error) {
	spec := p.GetDevfileRegistryDeploymentSpec()
	return deploy.SyncDeploymentSpecToCluster(p.deployContext, spec, deploy.DefaultDeploymentDiffOpts)
}

func computeDevfileRegistryUrl(endpoint string, cheCluster *orgv1.CheCluster) string {
	if cheCluster.Spec.Server.TlsSupport {
		return "https://" + endpoint
	} else {
		return "http://" + endpoint
	}
}
