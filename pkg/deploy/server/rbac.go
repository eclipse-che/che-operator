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

package server

import (
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	util "github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	commonPermissionsTemplateName       = "%s-cheworkspaces-clusterrole"
	namespacePermissionsTemplateName    = "%s-cheworkspaces-namespaces-clusterrole"
	devWorkspacePermissionsTemplateName = "%s-cheworkspaces-devworkspace-clusterrole"
)

// Create ClusterRole and ClusterRoleBinding for "che" service account.
// che-server uses "che" service account for creation RBAC for a user in his namespace.
func (s *CheServerReconciler) syncPermissions(ctx *chetypes.DeployContext) (bool, error) {
	policies := map[string][]rbacv1.PolicyRule{
		fmt.Sprintf(commonPermissionsTemplateName, ctx.CheCluster.Namespace):       s.getCommonPolicies(),
		fmt.Sprintf(namespacePermissionsTemplateName, ctx.CheCluster.Namespace):    s.getNamespaceEditorPolicies(),
		fmt.Sprintf(devWorkspacePermissionsTemplateName, ctx.CheCluster.Namespace): s.getDevWorkspacePolicies(),
	}

	for name, policy := range policies {
		if done, err := deploy.SyncClusterRoleToCluster(ctx, name, policy); !done {
			return false, err
		}

		if done, err := deploy.SyncClusterRoleBindingToCluster(ctx, name, constants.DefaultCheServiceAccountName, name); !done {
			return false, err
		}
	}

	for _, cheClusterRole := range ctx.CheCluster.Spec.Components.CheServer.ClusterRoles {
		cheClusterRole := strings.TrimSpace(cheClusterRole)
		if cheClusterRole != "" {
			if done, err := deploy.SyncClusterRoleBindingToCluster(ctx, cheClusterRole, constants.DefaultCheServiceAccountName, cheClusterRole); !done {
				return false, err
			}

			finalizer := s.getCRBFinalizerName(cheClusterRole)
			if err := deploy.AppendFinalizer(ctx, finalizer); err != nil {
				return false, err
			}
		}
	}

	// Delete abandoned CRBs
	for _, finalizer := range ctx.CheCluster.Finalizers {
		if strings.HasSuffix(finalizer, cheCRBFinalizerSuffix) {
			cheClusterRole := strings.TrimSuffix(finalizer, cheCRBFinalizerSuffix)
			if !util.Contains(ctx.CheCluster.Spec.Components.CheServer.ClusterRoles, cheClusterRole) {
				if done, err := deploy.Delete(ctx, types.NamespacedName{Name: cheClusterRole}, &rbacv1.ClusterRoleBinding{}); !done {
					return false, err
				}

				if err := deploy.DeleteFinalizer(ctx, finalizer); err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

func (s *CheServerReconciler) deletePermissions(ctx *chetypes.DeployContext) bool {
	names := []string{
		fmt.Sprintf(commonPermissionsTemplateName, ctx.CheCluster.Namespace),
		fmt.Sprintf(namespacePermissionsTemplateName, ctx.CheCluster.Namespace),
		fmt.Sprintf(devWorkspacePermissionsTemplateName, ctx.CheCluster.Namespace),
	}

	done := true

	for _, name := range names {
		if _, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRole{}); err != nil {
			done = false
			logrus.Errorf("Failed to delete ClusterRole '%s', cause: %v", name, err)
		}

		if _, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{}); err != nil {
			done = false
			logrus.Errorf("Failed to delete ClusterRoleBinding '%s', cause: %v", name, err)
		}
	}

	for _, name := range ctx.CheCluster.Spec.Components.CheServer.ClusterRoles {
		name := strings.TrimSpace(name)
		if name != "" {
			if _, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{}); err != nil {
				done = false
				logrus.Errorf("Failed to delete ClusterRoleBinding '%s', cause: %v", name, err)
			}

			// Removes any legacy CRB https://github.com/eclipse/che/issues/19506
			legacyName := ctx.CheCluster.Namespace + "-" + constants.DefaultCheServiceAccountName + "-" + name
			if _, err := deploy.Delete(ctx, types.NamespacedName{Name: legacyName}, &rbacv1.ClusterRoleBinding{}); err != nil {
				done = false
				logrus.Errorf("Failed to delete ClusterRoleBinding '%s', cause: %v", legacyName, err)
			}
		}
	}

	return done
}

func (s *CheServerReconciler) getDevWorkspacePolicies() []rbacv1.PolicyRule {
	k8sPolicies := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspaces", "devworkspacetemplates"},
			Verbs:     []string{"get", "create", "delete", "list", "update", "patch", "watch"},
		},
	}

	return k8sPolicies
}

func (s *CheServerReconciler) getNamespaceEditorPolicies() []rbacv1.PolicyRule {
	k8sPolicies := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update", "list"},
		},
	}

	openshiftPolicies := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projectrequests"},
			Verbs:     []string{"create", "update"},
		},
		{
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projects"},
			Verbs:     []string{"get", "list"},
		},
	}

	if infrastructure.IsOpenShift() {
		return append(k8sPolicies, openshiftPolicies...)
	}
	return k8sPolicies
}

func (s *CheServerReconciler) getCommonPolicies() []rbacv1.PolicyRule {
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
			Verbs:     []string{"get", "list", "watch", "create", "delete", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch", "create", "delete", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"get", "list", "create", "delete", "update", "patch"},
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

func (s *CheServerReconciler) getUserClusterRoles(ctx *chetypes.DeployContext) []string {
	return []string{
		fmt.Sprintf(commonPermissionsTemplateName, ctx.CheCluster.Namespace),
		fmt.Sprintf(devWorkspacePermissionsTemplateName, ctx.CheCluster.Namespace),
	}
}
