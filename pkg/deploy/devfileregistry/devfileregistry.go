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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DevfileRegistryReconciler struct {
	deploy.Reconcilable
}

func NewDevfileRegistryReconciler() *DevfileRegistryReconciler {
	return &DevfileRegistryReconciler{}
}

func (d *DevfileRegistryReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Server.ExternalDevfileRegistry {
		return reconcile.Result{}, true, nil
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

func (d *DevfileRegistryReconciler) Finalize(ctx *deploy.DeployContext) error {
	return nil
}

func (d *DevfileRegistryReconciler) syncService(ctx *deploy.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		deploy.DevfileRegistryName,
		[]string{"http"},
		[]int32{8080},
		deploy.DevfileRegistryName)
}

func (d *DevfileRegistryReconciler) syncConfigMap(ctx *deploy.DeployContext) (bool, error) {
	data, err := d.getConfigMapData(ctx)
	if err != nil {
		return false, err
	}
	return deploy.SyncConfigMapDataToCluster(ctx, deploy.DevfileRegistryName, data, deploy.DevfileRegistryName)
}

func (d *DevfileRegistryReconciler) exposeEndpoint(ctx *deploy.DeployContext) (string, bool, error) {
	return expose.Expose(
		ctx,
		deploy.DevfileRegistryName,
		ctx.CheCluster.Spec.Server.DevfileRegistryRoute,
		ctx.CheCluster.Spec.Server.DevfileRegistryIngress,
		d.createGatewayConfig())
}

func (d *DevfileRegistryReconciler) updateStatus(endpoint string, ctx *deploy.DeployContext) (bool, error) {
	var devfileRegistryURL string
	if ctx.CheCluster.Spec.Server.TlsSupport {
		devfileRegistryURL = "https://" + endpoint
	} else {
		devfileRegistryURL = "http://" + endpoint
	}

	if devfileRegistryURL != ctx.CheCluster.Status.DevfileRegistryURL {
		ctx.CheCluster.Status.DevfileRegistryURL = devfileRegistryURL
		if err := deploy.UpdateCheCRStatus(ctx, "status: Devfile Registry URL", devfileRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (d *DevfileRegistryReconciler) syncDeployment(ctx *deploy.DeployContext) (bool, error) {
	spec := d.getDevfileRegistryDeploymentSpec(ctx)
	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (d *DevfileRegistryReconciler) createGatewayConfig() *gateway.TraefikConfig {
	pathPrefix := "/" + deploy.DevfileRegistryName
	cfg := gateway.CreateCommonTraefikConfig(
		deploy.DevfileRegistryName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+deploy.DevfileRegistryName+":8080",
		[]string{pathPrefix})

	return cfg
}
