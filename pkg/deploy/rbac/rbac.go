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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
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

func (c *CheServerPermissionsReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// Create service account "che" for che-server component.
	// "che" is the one which token is used to create workspace objects.
	// Notice: Also we have on more "che-workspace" SA used by plugins like exec, terminal, metrics with limited privileges.
	done, err := deploy.SyncServiceAccountToCluster(ctx, constants.DefaultCheServiceAccountName)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	for _, cheClusterRole := range ctx.CheCluster.Spec.Components.CheServer.ClusterRoles {
		cheClusterRole := strings.TrimSpace(cheClusterRole)
		if cheClusterRole != "" {
			cheClusterRoleBindingName := cheClusterRole
			done, err := deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(ctx, cheClusterRoleBindingName, constants.DefaultCheServiceAccountName, cheClusterRole)
			if !done {
				return reconcile.Result{Requeue: true}, false, err
			}
		}
	}

	return reconcile.Result{}, true, err
}

func (c *CheServerPermissionsReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	done := true

	for _, cheClusterRole := range ctx.CheCluster.Spec.Components.CheServer.ClusterRoles {
		cheClusterRole := strings.TrimSpace(cheClusterRole)
		if cheClusterRole != "" {
			cheClusterRoleBindingName := cheClusterRole
			if err := deploy.ReconcileClusterRoleBindingFinalizer(ctx, cheClusterRoleBindingName); err != nil {
				done = false
				logrus.Errorf("Error deleting finalizer: %v", err)
			}

			// Removes any legacy CRB https://github.com/eclipse/che/issues/19506
			cheClusterRoleBindingName = deploy.GetLegacyUniqueClusterRoleBindingName(ctx, constants.DefaultCheServiceAccountName, cheClusterRole)
			if err := deploy.ReconcileLegacyClusterRoleBindingFinalizer(ctx, cheClusterRoleBindingName); err != nil {
				done = false
				logrus.Errorf("Error deleting finalizer: %v", err)
			}
		}
	}

	return done
}
