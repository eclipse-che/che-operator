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
package dashboard

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
)

const (
	// DashboardComponent which is supposed to be used for the naming related objects
	DashboardComponent = "che-dashboard"
)

type Dashboard struct {
	deployContext *deploy.DeployContext
}

func NewDashboard(deployContext *deploy.DeployContext) *Dashboard {
	return &Dashboard{
		deployContext: deployContext,
	}
}

func (d *Dashboard) SyncAll() (done bool, err error) {
	// Create a new dashboard service
	done, err = deploy.SyncServiceToCluster(d.deployContext, DashboardComponent, []string{"http"}, []int32{8080}, DashboardComponent)
	if !done {
		return false, err
	}

	// Expose dashboard service with route or ingress
	_, done, err = expose.ExposeWithHostPath(d.deployContext, DashboardComponent, d.deployContext.CheCluster.Spec.Server.CheHost,
		"/dashboard",
		d.deployContext.CheCluster.Spec.Server.CheServerRoute,
		d.deployContext.CheCluster.Spec.Server.CheServerIngress,
	)
	if !done {
		return false, err
	}

	// Deploy dashboard
	spec, err := d.getDashboardDeploymentSpec()
	if err != nil {
		return false, err
	}
	return deploy.SyncDeploymentSpecToCluster(d.deployContext, spec, deploy.DefaultDeploymentDiffOpts)
}
