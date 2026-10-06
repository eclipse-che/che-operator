//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
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
	reconciler.Reconcilable
}

func NewDashboardReconciler() *DashboardReconciler {
	return &DashboardReconciler{}
}

func (d *DashboardReconciler) getComponentName(cheCtx *chetypes.CheContext) string {
	return defaults.GetCheFlavor() + "-dashboard"
}

func (d *DashboardReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	// Create a new dashboard service
	if err := deploy.SyncServiceToCluster(cheCtx, d.getComponentName(cheCtx), []string{"http"}, []int32{8080}, d.getComponentName(cheCtx)); err != nil {
		return reconcile.Result{}, false, err
	}

	// Expose dashboard service with route or ingress
	_, done, err := expose.ExposeWithHostPath(cheCtx, d.getComponentName(cheCtx), cheCtx.CheHost,
		exposePath,
		d.createGatewayConfig(cheCtx),
	)
	if !done {
		return reconcile.Result{}, false, err
	}

	// we create dashboard SA in any case to keep a track on resources we access within it
	if err := deploy.SyncServiceAccountToCluster(cheCtx, DashboardSA); err != nil {
		return reconcile.Result{}, false, err
	}

	if err := deploy.SyncClusterRoleToCluster(cheCtx, d.getClusterRoleName(cheCtx), GetPrivilegedPoliciesRulesForKubernetes()); err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	if err := deploy.SyncClusterRoleBindingToCluster(cheCtx, d.getClusterRoleBindingName(cheCtx), DashboardSA, d.getClusterRoleName(cheCtx)); err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	if err := deploy.AppendFinalizer(cheCtx, ClusterPermissionsDashboardFinalizer); err != nil {
		return reconcile.Result{}, false, err
	}

	// Deploy dashboard
	spec, err := d.getDashboardDeploymentSpec(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	done, err = deploy.SyncDeploymentSpecToCluster(cheCtx, spec, deploy.DefaultDeploymentDiffOpts)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (d *DashboardReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	done := true
	if err := cheCtx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: d.getClusterRoleName(cheCtx)},
		&rbacv1.ClusterRole{},
	); err != nil {
		done = false
		logrus.Errorf("Failed to delete ClusterRole %s, cause: %v", d.getClusterRoleName(cheCtx), err)
	}

	if err := cheCtx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: d.getClusterRoleBindingName(cheCtx)},
		&rbacv1.ClusterRoleBinding{},
	); err != nil {
		done = false
		logrus.Errorf("Failed to delete ClusterRoleBinding %s, cause: %v", d.getClusterRoleBindingName(cheCtx), err)
	}

	if err := deploy.DeleteFinalizer(cheCtx, ClusterPermissionsDashboardFinalizer); err != nil {
		done = false
		logrus.Errorf("Error deleting finalizer: %v", err)
	}
	return done
}

func (d *DashboardReconciler) createGatewayConfig(cheCtx *chetypes.CheContext) *gateway.TraefikConfig {
	cfg := gateway.CreateCommonTraefikConfig(
		d.getComponentName(cheCtx),
		fmt.Sprintf("Path(`/`) || Path(`/f`) || PathPrefix(`%s`)", exposePath),
		10,
		"http://"+d.getComponentName(cheCtx)+":8080",
		[]string{})
	if cheCtx.CheCluster.IsAccessTokenConfigured() {
		cfg.AddAuthHeaderRewrite(d.getComponentName(cheCtx))
	}
	return cfg
}
