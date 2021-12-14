//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package rbac

import (
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CheServerPermissionsReconciler struct {
	deploy.Reconcilable
}

func NewCheServerPermissionsReconciler() *CheServerPermissionsReconciler {
	return &CheServerPermissionsReconciler{}
}

func (c *CheServerPermissionsReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	// Create service account "che" for che-server component.
	// "che" is the one which token is used to create workspace objects.
	// Notice: Also we have on more "che-workspace" SA used by plugins like exec, terminal, metrics with limited privileges.
	done, err := deploy.SyncServiceAccountToCluster(ctx, deploy.CheServiceAccountName)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	if len(ctx.CheCluster.Spec.Server.CheClusterRoles) > 0 {
		cheClusterRoles := strings.Split(ctx.CheCluster.Spec.Server.CheClusterRoles, ",")
		for _, cheClusterRole := range cheClusterRoles {
			cheClusterRole := strings.TrimSpace(cheClusterRole)
			cheClusterRoleBindingName := cheClusterRole
			done, err := deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(ctx, cheClusterRoleBindingName, deploy.CheServiceAccountName, cheClusterRole)
			if !done {
				return reconcile.Result{Requeue: true}, false, err
			}
		}
	}

	return reconcile.Result{}, true, err
}

func (c *CheServerPermissionsReconciler) Finalize(ctx *deploy.DeployContext) bool {
	done := true

	if len(ctx.CheCluster.Spec.Server.CheClusterRoles) > 0 {
		cheClusterRoles := strings.Split(ctx.CheCluster.Spec.Server.CheClusterRoles, ",")
		for _, cheClusterRole := range cheClusterRoles {
			cheClusterRole := strings.TrimSpace(cheClusterRole)
			cheClusterRoleBindingName := cheClusterRole
			if err := deploy.ReconcileClusterRoleBindingFinalizer(ctx, cheClusterRoleBindingName); err != nil {
				done = false
				logrus.Errorf("Error deleting finalizer: %v", err)
			}

			// Removes any legacy CRB https://github.com/eclipse/che/issues/19506
			cheClusterRoleBindingName = deploy.GetLegacyUniqueClusterRoleBindingName(ctx, deploy.CheServiceAccountName, cheClusterRole)
			if err := deploy.ReconcileLegacyClusterRoleBindingFinalizer(ctx, cheClusterRoleBindingName); err != nil {
				done = false
				logrus.Errorf("Error deleting finalizer: %v", err)
			}
		}
	}

	return done
}
