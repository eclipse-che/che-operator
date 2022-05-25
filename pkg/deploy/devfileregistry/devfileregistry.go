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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DevfileRegistryReconciler struct {
	deploy.Reconcilable
}

func NewDevfileRegistryReconciler() *DevfileRegistryReconciler {
	return &DevfileRegistryReconciler{}
}

func (d *DevfileRegistryReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Components.DevfileRegistry.DisableInternalRegistry {
		ctx.CheCluster.Status.DevfileRegistryURL = ""
		err := deploy.UpdateCheCRStatus(ctx, "DevfileRegistryURL", "")
		return reconcile.Result{}, err == nil, err
	}

	done, err := d.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	endpoint, done, err := d.exposeEndpoint(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = d.updateStatus(endpoint, ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = d.syncConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = d.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (d *DevfileRegistryReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (d *DevfileRegistryReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.DevfileRegistryName,
		[]string{"http"},
		[]int32{8080},
		constants.DevfileRegistryName)
}

func (d *DevfileRegistryReconciler) syncConfigMap(ctx *chetypes.DeployContext) (bool, error) {
	data, err := d.getConfigMapData(ctx)
	if err != nil {
		return false, err
	}
	return deploy.SyncConfigMapDataToCluster(ctx, constants.DevfileRegistryName, data, constants.DevfileRegistryName)
}

func (d *DevfileRegistryReconciler) exposeEndpoint(ctx *chetypes.DeployContext) (string, bool, error) {
	return expose.Expose(
		ctx,
		constants.DevfileRegistryName,
		d.createGatewayConfig())
}

func (d *DevfileRegistryReconciler) updateStatus(endpoint string, ctx *chetypes.DeployContext) (bool, error) {
	devfileRegistryURL := "https://" + endpoint
	if devfileRegistryURL != ctx.CheCluster.Status.DevfileRegistryURL {
		ctx.CheCluster.Status.DevfileRegistryURL = devfileRegistryURL
		if err := deploy.UpdateCheCRStatus(ctx, "status: Devfile Registry URL", devfileRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (d *DevfileRegistryReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec := d.getDevfileRegistryDeploymentSpec(ctx)
	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (d *DevfileRegistryReconciler) createGatewayConfig() *gateway.TraefikConfig {
	pathPrefix := "/" + constants.DevfileRegistryName
	cfg := gateway.CreateCommonTraefikConfig(
		constants.DevfileRegistryName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+constants.DevfileRegistryName+":8080",
		[]string{pathPrefix})

	return cfg
}
