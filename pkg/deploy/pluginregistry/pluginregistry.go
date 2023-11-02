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

package pluginregistry

import (
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
)

type PluginRegistryReconciler struct {
	deploy.Reconcilable
}

func NewPluginRegistryReconciler() *PluginRegistryReconciler {
	return &PluginRegistryReconciler{}
}

func (p *PluginRegistryReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Components.PluginRegistry.DisableInternalRegistry {
		ctx.CheCluster.Status.PluginRegistryURL = ""
		err := deploy.UpdateCheCRStatus(ctx, "PluginRegistryURL", "")
		return reconcile.Result{}, err == nil, err
	}

	done, err := p.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	endpoint, done, err := p.ExposeEndpoint(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.updateStatus(endpoint, ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (p *PluginRegistryReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (p *PluginRegistryReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.PluginRegistryName,
		[]string{"http"},
		[]int32{8080},
		constants.PluginRegistryName)
}

func (p *PluginRegistryReconciler) syncConfigMap(ctx *chetypes.DeployContext) (bool, error) {
	data, err := p.getConfigMapData(ctx)
	if err != nil {
		return false, err
	}
	return deploy.SyncConfigMapDataToCluster(ctx, constants.PluginRegistryName, data, constants.PluginRegistryName)
}

func (p *PluginRegistryReconciler) ExposeEndpoint(ctx *chetypes.DeployContext) (string, bool, error) {
	return expose.Expose(
		ctx,
		constants.PluginRegistryName,
		p.createGatewayConfig(ctx))
}

func (p *PluginRegistryReconciler) updateStatus(endpoint string, ctx *chetypes.DeployContext) (bool, error) {
	pluginRegistryURL := "https://" + endpoint

	// append the API version to plugin registry
	if !strings.HasSuffix(pluginRegistryURL, "/") {
		pluginRegistryURL = pluginRegistryURL + "/v3"
	} else {
		pluginRegistryURL = pluginRegistryURL + "v3"
	}

	if pluginRegistryURL != ctx.CheCluster.Status.PluginRegistryURL {
		ctx.CheCluster.Status.PluginRegistryURL = pluginRegistryURL
		if err := deploy.UpdateCheCRStatus(ctx, "status: Plugin Registry URL", pluginRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *PluginRegistryReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	if spec, err := p.getPluginRegistryDeploymentSpec(ctx); err != nil {
		return false, err
	} else {
		return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
	}
}

func (p *PluginRegistryReconciler) createGatewayConfig(ctx *chetypes.DeployContext) *gateway.TraefikConfig {
	pathPrefix := "/" + constants.PluginRegistryName
	cfg := gateway.CreateCommonTraefikConfig(
		constants.PluginRegistryName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+constants.PluginRegistryName+":8080",
		[]string{pathPrefix})

	return cfg
}
