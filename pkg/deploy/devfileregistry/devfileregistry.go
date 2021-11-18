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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
)

type DevfileRegistry struct {
	deployContext *deploy.DeployContext
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

	endpoint, done, err := p.ExposeEndpoint()
	if !done {
		return false, err
	}

	done, err = p.UpdateStatus(endpoint)
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

func (p *DevfileRegistry) ExposeEndpoint() (string, bool, error) {
	return expose.Expose(
		p.deployContext,
		deploy.DevfileRegistryName,
		p.deployContext.CheCluster.Spec.Server.DevfileRegistryRoute,
		p.deployContext.CheCluster.Spec.Server.DevfileRegistryIngress,
		p.createGatewayConfig())
}

func (p *DevfileRegistry) UpdateStatus(endpoint string) (bool, error) {
	var devfileRegistryURL string
	if p.deployContext.CheCluster.Spec.Server.TlsSupport {
		devfileRegistryURL = "https://" + endpoint
	} else {
		devfileRegistryURL = "http://" + endpoint
	}

	if devfileRegistryURL != p.deployContext.CheCluster.Status.DevfileRegistryURL {
		p.deployContext.CheCluster.Status.DevfileRegistryURL = devfileRegistryURL
		if err := deploy.UpdateCheCRStatus(p.deployContext, "status: Devfile Registry URL", devfileRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *DevfileRegistry) SyncDeployment() (bool, error) {
	spec := p.GetDevfileRegistryDeploymentSpec()
	return deploy.SyncDeploymentSpecToCluster(p.deployContext, spec, deploy.DefaultDeploymentDiffOpts)
}

func (p *DevfileRegistry) createGatewayConfig() *gateway.TraefikConfig {
	pathPrefix := "/" + deploy.DevfileRegistryName
	cfg := gateway.CreateCommonTraefikConfig(
		deploy.DevfileRegistryName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+deploy.DevfileRegistryName+":8080",
		[]string{pathPrefix})

	return cfg
}
