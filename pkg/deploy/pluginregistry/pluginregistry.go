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
package pluginregistry

import (
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
)

type PluginRegistry struct {
	deployContext *deploy.DeployContext
}

func NewPluginRegistry(deployContext *deploy.DeployContext) *PluginRegistry {
	return &PluginRegistry{
		deployContext: deployContext,
	}
}

func (p *PluginRegistry) SyncAll() (bool, error) {
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

func (p *PluginRegistry) SyncService() (bool, error) {
	return deploy.SyncServiceToCluster(
		p.deployContext,
		deploy.PluginRegistryName,
		[]string{"http"},
		[]int32{8080},
		deploy.PluginRegistryName)
}

func (p *PluginRegistry) SyncConfigMap() (bool, error) {
	data, err := p.GetConfigMapData()
	if err != nil {
		return false, err
	}
	return deploy.SyncConfigMapDataToCluster(p.deployContext, deploy.PluginRegistryName, data, deploy.PluginRegistryName)
}

func (p *PluginRegistry) ExposeEndpoint() (string, bool, error) {
	return expose.Expose(
		p.deployContext,
		deploy.PluginRegistryName,
		p.deployContext.CheCluster.Spec.Server.PluginRegistryRoute,
		p.deployContext.CheCluster.Spec.Server.PluginRegistryIngress,
		p.createGatewayConfig())
}

func (p *PluginRegistry) UpdateStatus(endpoint string) (bool, error) {
	var pluginRegistryURL string
	if p.deployContext.CheCluster.Spec.Server.TlsSupport {
		pluginRegistryURL = "https://" + endpoint
	} else {
		pluginRegistryURL = "http://" + endpoint
	}

	// append the API version to plugin registry
	if !strings.HasSuffix(pluginRegistryURL, "/") {
		pluginRegistryURL = pluginRegistryURL + "/v3"
	} else {
		pluginRegistryURL = pluginRegistryURL + "v3"
	}

	if pluginRegistryURL != p.deployContext.CheCluster.Status.PluginRegistryURL {
		p.deployContext.CheCluster.Status.PluginRegistryURL = pluginRegistryURL
		if err := deploy.UpdateCheCRStatus(p.deployContext, "status: Plugin Registry URL", pluginRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *PluginRegistry) SyncDeployment() (bool, error) {
	spec := p.GetPluginRegistryDeploymentSpec()
	return deploy.SyncDeploymentSpecToCluster(p.deployContext, spec, deploy.DefaultDeploymentDiffOpts)
}

func (p *PluginRegistry) createGatewayConfig() *gateway.TraefikConfig {
	pathPrefix := "/" + deploy.PluginRegistryName
	cfg := gateway.CreateCommonTraefikConfig(
		deploy.PluginRegistryName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+deploy.PluginRegistryName+":8080",
		[]string{pathPrefix})

	return cfg
}
