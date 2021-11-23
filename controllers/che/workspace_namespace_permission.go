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

package che

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
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
	// DevWorkspaceClusterRoleNameTemplate - manage DevWorkspace "cluster role" and "clusterrolebinding" template name
	DevWorkspaceClusterRoleNameTemplate = "%s-cheworkspaces-devworkspace-clusterrole"

	CheWorkspacesClusterPermissionsFinalizerName = "cheWorkspaces.clusterpermissions.finalizers.che.eclipse.org"
	NamespacesEditorPermissionsFinalizerName     = "namespaces-editor.permissions.finalizers.che.eclipse.org"
	DevWorkspacePermissionsFinalizerName         = "devWorkspace.permissions.finalizers.che.eclipse.org"
)

// Reconcile workspace permissions based on workspace strategy
func (r *CheClusterReconciler) reconcileWorkspacePermissions(deployContext *deploy.DeployContext) (bool, error) {
	done, err := r.delegateWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext)
	if !done {
		return false, err
	}

	done, err = r.delegateNamespaceEditorPermissions(deployContext)
	if !done {
		return false, err
	}

	done, err = r.delegateDevWorkspacePermissions(deployContext)
	if !done {
		return false, err
	}

	return true, nil
}

// Create cluster roles and cluster role bindings for "che" service account.
// che-server uses "che" service account for creation new workspaces and workspace components.
// Operator will create two cluster roles:
// - "<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" - cluster role to manage namespace(for Kubernetes platform)
//    or project(for Openshift platform) for new workspace.
// - "<workspace-namespace/project-name>-cheworkspaces-clusterrole" - cluster role to create and manage k8s objects required for
//    workspace components.
// Notice: After permission delegation che-server will create service account "che-workspace" ITSELF with
//         "exec" and "view" roles for each new workspace.
func (r *CheClusterReconciler) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *deploy.DeployContext) (bool, error) {
	сheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheWorkspacesClusterRoleBindingName := сheWorkspacesClusterRoleName

	// Create clusterrole +kubebuilder:storageversion"<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" to create k8s components for Che workspaces.
	done, err := deploy.SyncClusterRoleToCluster(deployContext, сheWorkspacesClusterRoleName, getWorkspacesPolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheWorkspacesClusterRoleBindingName, deploy.CheServiceAccountName, сheWorkspacesClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, CheWorkspacesClusterPermissionsFinalizerName)
	return err == nil, err
}

func (r *CheClusterReconciler) removeWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *deploy.DeployContext) (bool, error) {
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

func (r *CheClusterReconciler) delegateNamespaceEditorPermissions(deployContext *deploy.DeployContext) (bool, error) {
	сheNamespaceEditorClusterRoleName := fmt.Sprintf(CheNamespaceEditorClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheNamespaceEditorClusterRoleBindingName := сheNamespaceEditorClusterRoleName

	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-manage-namespaces" to manage namespace/projects for Che workspaces.
	done, err := deploy.SyncClusterRoleToCluster(deployContext, сheNamespaceEditorClusterRoleName, getNamespaceEditorPolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheNamespaceEditorClusterRoleBindingName, deploy.CheServiceAccountName, сheNamespaceEditorClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, NamespacesEditorPermissionsFinalizerName)
	return err == nil, err
}

func (r *CheClusterReconciler) removeNamespaceEditorPermissions(deployContext *deploy.DeployContext) (bool, error) {
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

func (r *CheClusterReconciler) delegateDevWorkspacePermissions(deployContext *deploy.DeployContext) (bool, error) {
	devWorkspaceClusterRoleName := fmt.Sprintf(DevWorkspaceClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	devWorkspaceClusterRoleBindingName := devWorkspaceClusterRoleName

	done, err := deploy.SyncClusterRoleToCluster(deployContext, devWorkspaceClusterRoleName, getDevWorkspacePolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, devWorkspaceClusterRoleBindingName, deploy.CheServiceAccountName, devWorkspaceClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, DevWorkspacePermissionsFinalizerName)
	return err == nil, err
}

func (r *CheClusterReconciler) removeDevWorkspacePermissions(deployContext *deploy.DeployContext) (bool, error) {
	devWorkspaceClusterRoleName := fmt.Sprintf(DevWorkspaceClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	devWorkspaceClusterRoleBindingName := devWorkspaceClusterRoleName

	done, err := deploy.Delete(deployContext, types.NamespacedName{Name: devWorkspaceClusterRoleName}, &rbac.ClusterRole{})
	if !done {
		return false, err
	}

	done, err = deploy.Delete(deployContext, types.NamespacedName{Name: devWorkspaceClusterRoleBindingName}, &rbac.ClusterRoleBinding{})
	if !done {
		return false, err
	}

	err = deploy.DeleteFinalizer(deployContext, DevWorkspacePermissionsFinalizerName)
	return err == nil, err
}

func (r *CheClusterReconciler) reconcileWorkspacePermissionsFinalizers(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		done, err := r.removeNamespaceEditorPermissions(deployContext)
		if !done {
			return false, err
		}

		done, err = r.removeDevWorkspacePermissions(deployContext)
		if !done {
			return false, err
		}

		return r.removeWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext)
	}

	return true, nil
}

func getDevWorkspacePolicies() []rbac.PolicyRule {
	k8sPolicies := []rbac.PolicyRule{
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspaces", "devworkspacetemplates"},
			Verbs:     []string{"get", "create", "delete", "list", "update", "patch"},
		},
	}

	return k8sPolicies
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
			Verbs:     []string{"get", "patch", "list", "update", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "create", "watch", "delete"},
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
			Verbs:     []string{"get", "patch", "list", "update", "create", "delete"},
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
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"list", "create", "watch", "get", "delete"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles"},
			Verbs:     []string{"get", "update", "create"},
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
			Verbs:     []string{"get", "update", "create"},
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
