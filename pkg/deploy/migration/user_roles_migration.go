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

package migration

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	userCommonClusterRoleTemplate = "%s-cheworkspaces-clusterrole"
	userDWClusterRoleTemplate     = "%s-cheworkspaces-devworkspace-clusterrole"
)

// UserRolesMigrator deletes legacy che-server-delegated RoleBindings in user namespaces.
// Legacy RoleBindings are those that lack the app.kubernetes.io/part-of=che.eclipse.org label.
type UserRolesMigrator struct {
	reconciler.Reconcilable

	migrationDone bool
}

// NewUserRolesMigrator creates a new UserRolesMigrator.
func NewUserRolesMigrator() *UserRolesMigrator {
	return &UserRolesMigrator{
		migrationDone: false,
	}
}

func (m *UserRolesMigrator) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if m.migrationDone {
		return reconcile.Result{}, true, nil
	}

	done, err := m.deleteLegacyRoleBindings(ctx)
	if done && err == nil {
		m.migrationDone = true
	}
	return reconcile.Result{}, done, err
}

func (m *UserRolesMigrator) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (m *UserRolesMigrator) deleteLegacyRoleBindings(ctx *chetypes.DeployContext) (bool, error) {
	cheNs := ctx.CheCluster.Namespace

	legacyRoleRefNames := map[string]bool{
		fmt.Sprintf(userCommonClusterRoleTemplate, cheNs): true,
		fmt.Sprintf(userDWClusterRoleTemplate, cheNs):     true,
	}

	// List all RoleBindings across all namespaces
	roleBindingList := &rbacv1.RoleBindingList{}
	if err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), roleBindingList, &client.ListOptions{}); err != nil {
		return false, err
	}

	for i := range roleBindingList.Items {
		rb := &roleBindingList.Items[i]

		// Only process RoleBindings that reference the legacy ClusterRoles
		if !legacyRoleRefNames[rb.RoleRef.Name] {
			continue
		}

		// Skip operator-owned RoleBindings (those with the part-of label)
		rbLabels := rb.GetLabels()
		if rbLabels != nil && rbLabels[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg {
			continue
		}

		// Delete the legacy RoleBinding
		if err := ctx.ClusterAPI.NonCachingClient.Delete(context.TODO(), rb); err != nil {
			return false, err
		}
		logrus.Infof("Deleted legacy RoleBinding '%s' in namespace '%s'", rb.Name, rb.Namespace)
	}

	return true, nil
}
