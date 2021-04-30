//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:	// Contributors:
//   Red Hat, Inc. - initial API and implementation	//   Red Hat, Inc. - initial API and implementation
//

package che

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// EditClusterRoleName - default "edit" cluster role. This role is pre-created on the cluster.
	// See more: https://kubernetes.io/blog/2017/10/using-rbac-generally-available-18/#granting-access-to-users
	EditClusterRoleName = "edit"
	// EditRoleBindingName - "edit" rolebinding for che-server.
	EditRoleBindingName = "che"
	// CheWorkspacesServiceAccount - service account created for Che workspaces.
	CheWorkspacesServiceAccount = "che-workspace"
	// ViewRoleBindingName - "view" role for "che-workspace" service account.
	ViewRoleBindingName = "che-workspace-view"
	// ExecRoleBindingName - "exec" role for "che-workspace" service account.
	ExecRoleBindingName = "che-workspace-exec"
	// CheNamespaceEditorClusterRoleNameTemplate - manage namespaces "cluster role" and "clusterrolebinding" template name
	CheNamespaceEditorClusterRoleNameTemplate = "%s-cheworkspaces-namespaces-clusterrole"
	// CheWorkspacesClusterRoleNameTemplate - manage workspaces "cluster role" and "clusterrolebinding" template name
	CheWorkspacesClusterRoleNameTemplate = "%s-cheworkspaces-clusterrole"

	CheWorkspacesClusterPermissionsFinalizerName = "cheWorkspaces.clusterpermissions.finalizers.che.eclipse.org"
	NamespacesEditorPermissionsFinalizerName     = "namespaces-editor.permissions.finalizers.che.eclipse.org"
)

// check if we can delegate cluster roles
// fall back to the "narrower" workspace namespace strategy otherwise
func (r *ReconcileChe) checkWorkspacePermissions(deployContext *deploy.DeployContext) (bool, error) {
	if util.IsWorkspacePermissionsInTheDifferNamespaceThanCheRequired(deployContext.CheCluster) {
		сheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
		exists, err := deploy.Get(deployContext, types.NamespacedName{Name: сheWorkspacesClusterRoleName}, &rbac.ClusterRole{})
		if err != nil {
			return false, err
		}
		if !exists {
			policies := getWorkspacesPolicies()
			if util.IsWorkspaceInDifferentNamespaceThanChe(deployContext.CheCluster) {
				policies = append(policies, getNamespaceEditorPolicies()...)
			}
			deniedRules, err := r.permissionChecker.GetNotPermittedPolicyRules(policies, "")
			if err != nil {
				return false, err
			}
			// fall back
			if len(deniedRules) > 0 {
				logrus.Warnf("Not enough permissions to start a workspace in dedicated namespace. Denied policies: %v", deniedRules)
				logrus.Warnf("Fall back to '%s' namespace for workspaces.", deployContext.CheCluster.Namespace)
				delete(deployContext.CheCluster.Spec.Server.CustomCheProperties, "CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT")
				deployContext.CheCluster.Spec.Server.WorkspaceNamespaceDefault = deployContext.CheCluster.Namespace
				err := r.UpdateCheCRSpec(deployContext.CheCluster, "Default namespace for workspaces", deployContext.CheCluster.Namespace)
				if err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

// Reconcile workspace permissions based on workspace strategy
func (r *ReconcileChe) reconcileWorkspacePermissions(deployContext *deploy.DeployContext) (bool, error) {
	if util.IsWorkspacePermissionsInTheDifferNamespaceThanCheRequired(deployContext.CheCluster) {
		// Delete permission set for configuration "same namespace for Che and workspaces".
		done, err := r.removeWorkspacePermissionsInSameNamespaceWithChe(deployContext)
		if !done {
			return false, err
		}

		// Add workspaces cluster permission finalizer to the CR if deletion timestamp is 0.
		// Or delete workspaces cluster permission set and finalizer from CR if deletion timestamp is not 0.
		done, err = r.delegateWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext)
		if !done {
			return false, err
		}
	} else {
		// Delete workspaces cluster permission set and finalizer from CR if deletion timestamp is not 0.
		done, err := r.removeWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext)
		if !done {
			return false, err
		}

		done, err = r.delegateWorkspacePermissionsInTheSameNamespaceWithChe(deployContext)
		if !done {
			return false, err
		}
	}

	if util.IsWorkspaceInDifferentNamespaceThanChe(deployContext.CheCluster) {
		done, err := r.delegateNamespaceEditorPermissions(deployContext)
		if !done {
			return false, err
		}
	} else {
		done, err := r.removeNamespaceEditorPermissions(deployContext)
		if !done {
			return false, err
		}
	}

	return true, nil
}

// delegateWorkspacePermissionsInTheSameNamespaceWithChe - creates "che-workspace" service account(for Che workspaces) and
// delegates "che-operator" SA permissions to the service accounts: "che" and "che-workspace".
// Also this method binds "edit" default k8s clusterrole using rolebinding to "che" SA.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheSameNamespaceWithChe(deployContext *deploy.DeployContext) (bool, error) {
	// Create "che-workspace" service account.
	// Che workspace components use this service account.
	done, err := deploy.SyncServiceAccountToCluster(deployContext, CheWorkspacesServiceAccount)
	if !done {
		return false, err
	}

	// Create view role for "che-workspace" service account.
	// This role used by exec terminals, tasks, metric che-theia plugin and so on.
	done, err = deploy.SyncViewRoleToCluster(deployContext)
	if !done {
		return false, err
	}

	done, err = deploy.SyncRoleBindingToCluster(deployContext, ViewRoleBindingName, CheWorkspacesServiceAccount, deploy.ViewRoleName, "Role")
	if !done {
		return false, err
	}

	// Create exec role for "che-workspaces" service account.
	// This role used by exec terminals, tasks and so on.
	done, err = deploy.SyncExecRoleToCluster(deployContext)
	if !done {
		return false, err
	}

	done, err = deploy.SyncRoleBindingToCluster(deployContext, ExecRoleBindingName, CheWorkspacesServiceAccount, deploy.ExecRoleName, "Role")
	if !done {
		return false, err
	}

	// Bind "edit" cluster role for "che" service account.
	// che-operator doesn't create "edit" role. This role is pre-created on the cluster.
	// Warning: operator binds clusterrole using rolebinding(not clusterrolebinding).
	// That's why "che" service account has got permissions only in the one namespace!
	// So permissions are binding in "non-cluster" scope.
	done, err = deploy.SyncRoleBindingToCluster(deployContext, EditRoleBindingName, CheServiceAccountName, EditClusterRoleName, "ClusterRole")
	if !done {
		return false, err
	}

	return true, nil
}

// removeWorkspacePermissionsInSameNamespaceWithChe - removes workspaces in same namespace with Che role and rolebindings.
func (r *ReconcileChe) removeWorkspacePermissionsInSameNamespaceWithChe(deployContext *deploy.DeployContext) (bool, error) {
	done, err := deploy.DeleteNamespacedObject(deployContext, deploy.ExecRoleName, &rbac.Role{})
	if !done {
		return false, err
	}

	done, err = deploy.DeleteNamespacedObject(deployContext, ExecRoleBindingName, &rbac.RoleBinding{})
	if !done {
		return false, err
	}

	done, err = deploy.DeleteNamespacedObject(deployContext, deploy.ViewRoleName, &rbac.Role{})
	if !done {
		return false, err
	}

	done, err = deploy.DeleteNamespacedObject(deployContext, ViewRoleBindingName, &rbac.RoleBinding{})
	if !done {
		return false, err
	}

	done, err = deploy.DeleteNamespacedObject(deployContext, EditRoleBindingName, &rbac.RoleBinding{})
	if !done {
		return false, err
	}

	return true, nil
}

// Create cluster roles and cluster role bindings for "che" service account.
// che-server uses "che" service account for creation new workspaces and workspace components.
// Operator will create two cluster roles:
// - "<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" - cluster role to mange namespace(for Kubernetes platform)
//    or project(for Openshift platform) for new workspace.
// - "<workspace-namespace/project-name>-cheworkspaces-clusterrole" - cluster role to create and manage k8s objects required for
//    workspace components.
// Notice: After permission delegation che-server will create service account "che-workspace" ITSELF with
//         "exec" and "view" roles for each new workspace.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *deploy.DeployContext) (bool, error) {
	сheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheWorkspacesClusterRoleBindingName := сheWorkspacesClusterRoleName

	// Create clusterrole "<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" to create k8s components for Che workspaces.
	done, err := deploy.SyncClusterRoleToCluster(deployContext, сheWorkspacesClusterRoleName, getWorkspacesPolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheWorkspacesClusterRoleBindingName, CheServiceAccountName, сheWorkspacesClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, CheWorkspacesClusterPermissionsFinalizerName)
	return err == nil, err
}

func (r *ReconcileChe) removeWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *deploy.DeployContext) (bool, error) {
	cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	cheWorkspacesClusterRoleBindingName := cheWorkspacesClusterRoleName

	done, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesClusterRoleName}, &rbac.ClusterRole{})
	if !done {
		return false, err
	}

	done, err = deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesClusterRoleBindingName}, &rbac.ClusterRoleBinding{})
	if !done {
		return false, err
	}

	err = deploy.DeleteFinalizer(deployContext, CheWorkspacesClusterPermissionsFinalizerName)
	return err == nil, err
}

func (r *ReconcileChe) delegateNamespaceEditorPermissions(deployContext *deploy.DeployContext) (bool, error) {
	сheNamespaceEditorClusterRoleName := fmt.Sprintf(CheNamespaceEditorClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheNamespaceEditorClusterRoleBindingName := сheNamespaceEditorClusterRoleName

	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-manage-namespaces" to manage namespace/projects for Che workspaces.
	done, err := deploy.SyncClusterRoleToCluster(deployContext, сheNamespaceEditorClusterRoleName, getNamespaceEditorPolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheNamespaceEditorClusterRoleBindingName, CheServiceAccountName, сheNamespaceEditorClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, NamespacesEditorPermissionsFinalizerName)
	return err == nil, err
}

func (r *ReconcileChe) removeNamespaceEditorPermissions(deployContext *deploy.DeployContext) (bool, error) {
	cheNamespaceEditorClusterRoleName := fmt.Sprintf(CheNamespaceEditorClusterRoleNameTemplate, deployContext.CheCluster.Namespace)

	done, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheNamespaceEditorClusterRoleName}, &rbac.ClusterRole{})
	if !done {
		return false, err
	}

	done, err = deploy.Delete(deployContext, types.NamespacedName{Name: cheNamespaceEditorClusterRoleName}, &rbac.ClusterRoleBinding{})
	if !done {
		return false, err
	}

	err = deploy.DeleteFinalizer(deployContext, NamespacesEditorPermissionsFinalizerName)
	return err == nil, err
}

func (r *ReconcileChe) reconcileWorkspacePermissionsFinalizers(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		done, err := r.removeNamespaceEditorPermissions(deployContext)
		if !done {
			return false, err
		}

		return r.removeWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext)
	}

	return true, nil
}

func getNamespaceEditorPolicies() []rbac.PolicyRule {
	k8sPolicies := []rbac.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update", "list"},
		},
	}

	openshiftPolicies := []rbac.PolicyRule{
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

	if util.IsOpenShift {
		return append(k8sPolicies, openshiftPolicies...)
	}
	return k8sPolicies
}

func getWorkspacesPolicies() []rbac.PolicyRule {
	k8sPolicies := []rbac.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"serviceaccounts"},
			Verbs:     []string{"get", "create", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims", "configmaps"},
			Verbs:     []string{"list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"list", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "create", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "create", "list", "watch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"create", "list", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "create", "delete"},
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
			Verbs:     []string{"get", "create", "list", "watch", "patch", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"list", "get", "patch", "delete"},
		},
		{
			APIGroups: []string{"extensions"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"list", "create", "watch", "get", "delete"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles"},
			Verbs:     []string{"get", "create"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"rolebindings"},
			Verbs:     []string{"get", "update", "create"},
		},
		{
			APIGroups: []string{"metrics.k8s.io"},
			Resources: []string{"pods", "nodes"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
	openshiftPolicies := []rbac.PolicyRule{
		{
			APIGroups: []string{"route.openshift.io"},
			Resources: []string{"routes"},
			Verbs:     []string{"list", "create", "delete"},
		},
		{
			APIGroups: []string{"authorization.openshift.io"},
			Resources: []string{"roles"},
			Verbs:     []string{"get", "create"},
		},
		{
			APIGroups: []string{"authorization.openshift.io"},
			Resources: []string{"rolebindings"},
			Verbs:     []string{"get", "update", "create"},
		},
	}

	if util.IsOpenShift {
		return append(k8sPolicies, openshiftPolicies...)
	}
	return k8sPolicies
}
