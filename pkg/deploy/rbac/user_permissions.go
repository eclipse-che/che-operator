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

package rbac

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// CheUserPermissionsTemplateName - template for ClusterRole and ClusterRoleBinding names
	CheUserPermissionsTemplateName = "%s-cheworkspaces-clusterrole"

	// Legacy permissions
	CheUserNamespaceEditorPermissionsTemplateName = "%s-cheworkspaces-namespaces-clusterrole"
	CheUserDevWorkspacePermissionsTemplateName    = "%s-cheworkspaces-devworkspace-clusterrole"
)

type UserPermissionsReconciler struct {
	deploy.Reconcilable
}

func NewUserPermissionsReconciler() *UserPermissionsReconciler {
	return &UserPermissionsReconciler{}
}

func (up *UserPermissionsReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// Create ClusterRole and ClusterRoleBinding for "che" service account.
	// che-server uses "che" service account for creation RBAC for a user in his namespace.
	name := fmt.Sprintf(CheUserPermissionsTemplateName, ctx.CheCluster.Namespace)

	if done, err := deploy.SyncClusterRoleToCluster(ctx, name, up.getUserPolicies()); !done {
		return reconcile.Result{}, false, err
	}

	if done, err := deploy.SyncClusterRoleBindingToCluster(ctx, name, constants.DefaultCheServiceAccountName, name); !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (up *UserPermissionsReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	done := true

	if completed := up.removePermissions(ctx, CheUserPermissionsTemplateName); !completed {
		done = false
	}

	// Remove legacy permissions
	if completed := up.removePermissions(ctx, CheUserDevWorkspacePermissionsTemplateName); !completed {
		done = false
	}

	if completed := up.removePermissions(ctx, CheUserNamespaceEditorPermissionsTemplateName); !completed {
		done = false
	}

	return done
}

func (up *UserPermissionsReconciler) removePermissions(ctx *chetypes.DeployContext, templateName string) bool {
	completed := true

	name := fmt.Sprintf(templateName, ctx.CheCluster.Namespace)
	if done, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRole{}); !done {
		completed = false
		logrus.Errorf("Failed to delete ClusterRole '%s', cause: %v", name, err)
	}

	if done, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{}); !done {
		completed = false
		logrus.Errorf("Failed to delete ClusterRoleBinding '%s', cause: %v", name, err)
	}

	return completed
}

func (up *UserPermissionsReconciler) getUserPolicies() []rbacv1.PolicyRule {
	k8sPolicies := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"serviceaccounts"},
			Verbs:     []string{"get", "watch", "create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"get", "create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "list", "watch", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"get", "list", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"secrets"},
			Verbs:     []string{"list"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch", "create", "patch", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"get", "list", "patch", "delete"},
		},
		{
			APIGroups: []string{"extensions"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch", "create", "delete"},
		},
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch", "create", "delete"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles"},
			Verbs:     []string{"get", "create", "update"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"rolebindings"},
			Verbs:     []string{"get", "create", "update"},
		},
		{
			APIGroups: []string{"metrics.k8s.io"},
			Resources: []string{"pods", "nodes"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"watch", "list"},
		},
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspaces", "devworkspacetemplates"},
			Verbs:     []string{"get", "create", "delete", "list", "update", "patch", "watch"},
		},
	}
	openshiftPolicies := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"route.openshift.io"},
			Resources: []string{"routes"},
			Verbs:     []string{"get", "list", "create", "delete"},
		},
		{
			APIGroups: []string{"authorization.openshift.io"},
			Resources: []string{"roles"},
			Verbs:     []string{"get", "create", "update"},
		},
		{
			APIGroups: []string{"authorization.openshift.io"},
			Resources: []string{"rolebindings"},
			Verbs:     []string{"get", "create", "update"},
		},
		{
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projects"},
			Verbs:     []string{"get"},
		},
	}

	if infrastructure.IsOpenShift() {
		return append(k8sPolicies, openshiftPolicies...)
	}
	return k8sPolicies
}
