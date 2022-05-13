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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	rbacv1 "k8s.io/api/rbac/v1"
)

const ClusterPermissionsDashboardFinalizer = "dashboard.clusterpermissions.finalizers.che.eclipse.org"

const DashboardSA = "che-dashboard"
const DashboardSAClusterRoleTemplate = "%s-che-dashboard"
const DashboardSAClusterRoleBindingTemplate = "%s-che-dashboard"

func GetPrivilegedPoliciesRulesForKubernetes() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspaces"},
			Verbs:     []string{"create", "update", "patch", "get", "watch", "list", "delete"},
		},
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspacetemplates"},
			Verbs:     []string{"create", "get", "list", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update", "list"},
		},
		{
			APIGroups: []string{"org.eclipse.che"},
			Resources: []string{"checlusters"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	if !infrastructure.IsOpenShift() {
		rules = append(rules,
			// on Kubernetes, Dashboard stores user preferences in secrets with SA
			// until native auth is not implemented there as well
			rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "create", "update", "list"},
			})
	}

	return rules
}

func (d *DashboardReconciler) getClusterRoleName(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf(DashboardSAClusterRoleTemplate, ctx.CheCluster.Namespace)
}

func (d *DashboardReconciler) getClusterRoleBindingName(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf(DashboardSAClusterRoleBindingTemplate, ctx.CheCluster.Namespace)
}
