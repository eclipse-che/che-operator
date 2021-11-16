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

package dashboard

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/util"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	exposePath = "/dashboard/"
)

var (
	log = ctrl.Log.WithName("dashboard")
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

func (d *Dashboard) GetComponentName() string {
	return d.component
}

func (d *Dashboard) Reconcile() (done bool, err error) {
	// Create a new dashboard service
	done, err = deploy.SyncServiceToCluster(d.deployContext, d.component, []string{"http"}, []int32{8080}, d.component)
	if !done {
		return false, err
	}

	// Expose dashboard service with route or ingress
	_, done, err = expose.ExposeWithHostPath(d.deployContext, d.component, d.deployContext.CheCluster.Spec.Server.CheHost,
		exposePath,
		d.deployContext.CheCluster.Spec.Server.DashboardRoute,
		d.deployContext.CheCluster.Spec.Server.DashboardIngress,
		d.createGatewayConfig(),
	)
	if !done {
		return false, err
	}

	// we create dashboard SA in any case to keep a track on resources we access withing it
	done, err = deploy.SyncServiceAccountToCluster(d.deployContext, DashboardSA)
	if !done {
		return done, err
	}

	// on Kubernetes Dashboard needs privileged SA to work with user's objects
	// for time being until Kubernetes did not get authentication
	if !util.IsOpenShift {
		done, err = deploy.SyncClusterRoleToCluster(d.deployContext, d.getClusterRoleName(), GetPrivilegedPoliciesRulesForKubernetes())
		if !done {
			return false, err
		}

		done, err = deploy.SyncClusterRoleBindingToCluster(d.deployContext, d.getClusterRoleBindingName(), DashboardSA, d.getClusterRoleName())
		if !done {
			return false, err
		}

		err = deploy.AppendFinalizer(d.deployContext, ClusterPermissionsDashboardFinalizer)
		if err != nil {
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

func (d *Dashboard) Finalize() (done bool, err error) {
	done, err = deploy.Delete(d.deployContext, types.NamespacedName{Name: d.getClusterRoleName()}, &rbacv1.ClusterRole{})
	if !done {
		return false, err
	}

	done, err = deploy.Delete(d.deployContext, types.NamespacedName{Name: d.getClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{})
	if !done {
		return false, err
	}

	err = deploy.DeleteFinalizer(d.deployContext, ClusterPermissionsDashboardFinalizer)
	return err == nil, err
}

func (d *Dashboard) createGatewayConfig() *gateway.TraefikConfig {
	cfg := gateway.CreateCommonTraefikConfig(
		d.component,
		fmt.Sprintf("PathPrefix(`%s`)", exposePath),
		10,
		"http://"+d.component+":8080",
		[]string{})
	if util.IsOpenShift && d.deployContext.CheCluster.IsNativeUserModeEnabled() {
		cfg.AddAuthHeaderRewrite(d.component)
	}
	return cfg
}
