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

type WorkspacePermissionsReconciler struct {
	deploy.Reconcilable
}

func NewWorkspacePermissionsReconciler() *WorkspacePermissionsReconciler {
	return &WorkspacePermissionsReconciler{}
}

func (wp *WorkspacePermissionsReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	done, err := wp.delegateWorkspacePermissionsInTheDifferNamespaceThanChe(ctx)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	done, err = wp.delegateNamespaceEditorPermissions(ctx)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	done, err = wp.delegateDevWorkspacePermissions(ctx)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (wp *WorkspacePermissionsReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	done := true

	if completed := wp.removeNamespaceEditorPermissions(ctx); !completed {
		done = false
	}

	if completed := wp.removeDevWorkspacePermissions(ctx); !completed {
		done = false
	}

	if completed := wp.removeWorkspacePermissionsInTheDifferNamespaceThanChe(ctx); !completed {
		done = false
	}

	return done
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
func (wp *WorkspacePermissionsReconciler) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *chetypes.DeployContext) (bool, error) {
	сheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheWorkspacesClusterRoleBindingName := сheWorkspacesClusterRoleName

	// Create clusterrole +kubebuilder:storageversion"<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" to create k8s components for Che workspaces.
	done, err := deploy.SyncClusterRoleToCluster(deployContext, сheWorkspacesClusterRoleName, wp.getWorkspacesPolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheWorkspacesClusterRoleBindingName, constants.DefaultCheServiceAccountName, сheWorkspacesClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, CheWorkspacesClusterPermissionsFinalizerName)
	return err == nil, err
}

func (wp *WorkspacePermissionsReconciler) removeWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *chetypes.DeployContext) bool {
	done := true

	cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	cheWorkspacesClusterRoleBindingName := cheWorkspacesClusterRoleName

	if _, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesClusterRoleName}, &rbacv1.ClusterRole{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete Che Workspaces ClusterRole '%s', cause: %v", cheWorkspacesClusterRoleName, err)
	}

	if _, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete Che Workspace ClusterRoleBinding '%s', cause: %v", cheWorkspacesClusterRoleBindingName, err)
	}

	if err := deploy.DeleteFinalizer(deployContext, CheWorkspacesClusterPermissionsFinalizerName); err != nil {
		done = false
		logrus.Errorf("Error deleting finalizer: %v", err)
	}

	return done
}

func (wp *WorkspacePermissionsReconciler) delegateNamespaceEditorPermissions(deployContext *chetypes.DeployContext) (bool, error) {
	сheNamespaceEditorClusterRoleName := fmt.Sprintf(CheNamespaceEditorClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheNamespaceEditorClusterRoleBindingName := сheNamespaceEditorClusterRoleName

	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-manage-namespaces" to manage namespace/projects for Che workspaces.
	done, err := deploy.SyncClusterRoleToCluster(deployContext, сheNamespaceEditorClusterRoleName, wp.getNamespaceEditorPolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheNamespaceEditorClusterRoleBindingName, constants.DefaultCheServiceAccountName, сheNamespaceEditorClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, NamespacesEditorPermissionsFinalizerName)
	return err == nil, err
}

func (wp *WorkspacePermissionsReconciler) removeNamespaceEditorPermissions(deployContext *chetypes.DeployContext) bool {
	done := true

	cheNamespaceEditorClusterRoleName := fmt.Sprintf(CheNamespaceEditorClusterRoleNameTemplate, deployContext.CheCluster.Namespace)

	if _, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheNamespaceEditorClusterRoleName}, &rbacv1.ClusterRole{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete Editor ClusterRole '%s', cause: %v", cheNamespaceEditorClusterRoleName, err)
	}

	if _, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheNamespaceEditorClusterRoleName}, &rbacv1.ClusterRoleBinding{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete Editor ClusterRoleBinding '%s', cause: %v", cheNamespaceEditorClusterRoleName, err)
	}

	if err := deploy.DeleteFinalizer(deployContext, NamespacesEditorPermissionsFinalizerName); err != nil {
		done = false
		logrus.Errorf("Error deleting finalizer: %v", err)
	}

	return done
}

func (wp *WorkspacePermissionsReconciler) delegateDevWorkspacePermissions(deployContext *chetypes.DeployContext) (bool, error) {
	devWorkspaceClusterRoleName := fmt.Sprintf(DevWorkspaceClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	devWorkspaceClusterRoleBindingName := devWorkspaceClusterRoleName

	done, err := deploy.SyncClusterRoleToCluster(deployContext, devWorkspaceClusterRoleName, wp.getDevWorkspacePolicies())
	if !done {
		return false, err
	}

	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, devWorkspaceClusterRoleBindingName, constants.DefaultCheServiceAccountName, devWorkspaceClusterRoleName)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(deployContext, DevWorkspacePermissionsFinalizerName)
	return err == nil, err
}

func (wp *WorkspacePermissionsReconciler) removeDevWorkspacePermissions(deployContext *chetypes.DeployContext) bool {
	done := true

	devWorkspaceClusterRoleName := fmt.Sprintf(DevWorkspaceClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	devWorkspaceClusterRoleBindingName := devWorkspaceClusterRoleName

	if _, err := deploy.Delete(deployContext, types.NamespacedName{Name: devWorkspaceClusterRoleName}, &rbacv1.ClusterRole{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete DevWorkspace ClusterRole '%s', cause: %v", devWorkspaceClusterRoleName, err)
	}

	if _, err := deploy.Delete(deployContext, types.NamespacedName{Name: devWorkspaceClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{}); err != nil {
		done = false
		logrus.Errorf("Failed to delete DevWorkspace ClusterRoleBinding '%s', cause: %v", devWorkspaceClusterRoleName, err)
	}

	if err := deploy.DeleteFinalizer(deployContext, DevWorkspacePermissionsFinalizerName); err != nil {
		done = false
		logrus.Errorf("Error deleting finalizer: %v", err)
	}

	return done
}

func (wp *WorkspacePermissionsReconciler) getDevWorkspacePolicies() []rbacv1.PolicyRule {
	k8sPolicies := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspaces", "devworkspacetemplates"},
			Verbs:     []string{"get", "create", "delete", "list", "update", "patch", "watch"},
		},
	}

	return k8sPolicies
}

func (wp *WorkspacePermissionsReconciler) getNamespaceEditorPolicies() []rbacv1.PolicyRule {
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

func (c *WorkspacePermissionsReconciler) getWorkspacesPolicies() []rbacv1.PolicyRule {
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
