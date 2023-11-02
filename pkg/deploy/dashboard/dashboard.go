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

package dashboard

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	exposePath = "/dashboard/"
)

var (
	log = ctrl.Log.WithName("dashboard")
)

type DashboardReconciler struct {
	deploy.Reconcilable
}

func NewDashboardReconciler() *DashboardReconciler {
	return &DashboardReconciler{}
}

func (d *DashboardReconciler) getComponentName(ctx *chetypes.DeployContext) string {
	return defaults.GetCheFlavor() + "-dashboard"
}

func (d *DashboardReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// Create a new dashboard service
	done, err := deploy.SyncServiceToCluster(ctx, d.getComponentName(ctx), []string{"http"}, []int32{8080}, d.getComponentName(ctx))
	if !done {
		return reconcile.Result{}, false, err
	}

	// Expose dashboard service with route or ingress
	_, done, err = expose.ExposeWithHostPath(ctx, d.getComponentName(ctx), ctx.CheHost,
		exposePath,
		d.createGatewayConfig(ctx),
	)
	if !done {
		return reconcile.Result{}, false, err
	}

	// we create dashboard SA in any case to keep a track on resources we access withing it
	done, err = deploy.SyncServiceAccountToCluster(ctx, DashboardSA)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = deploy.SyncClusterRoleToCluster(ctx, d.getClusterRoleName(ctx), GetPrivilegedPoliciesRulesForKubernetes())
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(ctx, d.getClusterRoleBindingName(ctx), DashboardSA, d.getClusterRoleName(ctx))
	if !done {
		return reconcile.Result{}, false, err
	}

	err = deploy.AppendFinalizer(ctx, ClusterPermissionsDashboardFinalizer)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	// Deploy dashboard
	spec, err := d.getDashboardDeploymentSpec(ctx)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	done, err = deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (d *DashboardReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	done := true
	if _, err := deploy.Delete(ctx, types.NamespacedName{Name: d.getClusterRoleName(ctx)}, &rbacv1.ClusterRole{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete ClusterRole %s, cause: %v", d.getClusterRoleName(ctx), err)
	}

	if _, err := deploy.Delete(ctx, types.NamespacedName{Name: d.getClusterRoleBindingName(ctx)}, &rbacv1.ClusterRoleBinding{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete ClusterRoleBinding %s, cause: %v", d.getClusterRoleBindingName(ctx), err)
	}

	if err := deploy.DeleteFinalizer(ctx, ClusterPermissionsDashboardFinalizer); err != nil {
		done = false
		logrus.Errorf("Error deleting finalizer: %v", err)
	}
	return done
}

func (d *DashboardReconciler) createGatewayConfig(ctx *chetypes.DeployContext) *gateway.TraefikConfig {
	cfg := gateway.CreateCommonTraefikConfig(
		d.getComponentName(ctx),
		fmt.Sprintf("Path(`/`, `/f`) || PathPrefix(`%s`)", exposePath),
		10,
		"http://"+d.getComponentName(ctx)+":8080",
		[]string{})
	if ctx.CheCluster.IsAccessTokenConfigured() {
		cfg.AddAuthHeaderRewrite(d.getComponentName(ctx))
	}
	return cfg
}
