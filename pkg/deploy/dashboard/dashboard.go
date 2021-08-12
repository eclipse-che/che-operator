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
	"github.com/eclipse-che/che-operator/pkg/util"
)

type Dashboard struct {
	deployContext *deploy.DeployContext
	component     string
}

func NewDashboard(deployContext *deploy.DeployContext) *Dashboard {
	return &Dashboard{
		deployContext: deployContext,
		component:     deploy.DefaultCheFlavor(deployContext.CheCluster) + "-dashboard",
	}
}

func (d *Dashboard) SyncAll() (done bool, err error) {
	// Create a new dashboard service
	done, err = deploy.SyncServiceToCluster(d.deployContext, d.component, []string{"http"}, []int32{8080}, d.component)
	if !done {
		return false, err
	}

	// Expose dashboard service with route or ingress
	_, done, err = expose.ExposeWithHostPath(d.deployContext, d.component, d.deployContext.CheCluster.Spec.Server.CheHost,
		"/dashboard/",
		d.deployContext.CheCluster.Spec.Server.DashboardRoute,
		d.deployContext.CheCluster.Spec.Server.DashboardIngress,
	)
	if !done {
		return false, err
	}

	// we create dashboard SA in any case to keep a track on resources we access withing it
	done, err = deploy.SyncServiceAccountToCluster(d.deployContext, DashboardSA)
	if !done {
		return false, err
	}
	// on Kubernetes Dashboard needs privileged SA to work with user's objects
	// for time being until Kubernetes did not get authentication
	if !util.IsOpenShift {
		done, err = deploy.SyncClusterRoleToCluster(d.deployContext, DashboardSAClusterRole, GetPrivilegedPoliciesRulesForKubernetes())
		if !done {
			return false, err
		}

		done, err = deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(d.deployContext, DashboardSAClusterRoleBinding, DashboardSA, DashboardSAClusterRole)
		if !done {
			return false, err
		}
	}

	// Deploy dashboard
	spec, err := d.getDashboardDeploymentSpec()
	if err != nil {
		return false, err
	}
	return deploy.SyncDeploymentSpecToCluster(d.deployContext, spec, deploy.DefaultDeploymentDiffOpts)
}

func (d *Dashboard) GetComponentName() string {
	return d.component
}
