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
	"time"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	// CheWorkspacesNamespaceClusterRoleNameTemplate - manage namespaces "cluster role" and "clusterrolebinding" template name
	CheWorkspacesNamespaceClusterRoleNameTemplate = "%s-cheworkspaces-namespaces-clusterrole"
	// CheWorkspacesClusterRoleNameTemplate - manage workspaces "cluster role" and "clusterrolebinding" template name
	CheWorkspacesClusterRoleNameTemplate = "%s-cheworkspaces-clusterrole"
)

// delegateWorkspacePermissionsInTheSameNamespaceWithChe - creates "che-workspace" service account(for Che workspaces) and
// delegates "che-operator" SA permissions to the service accounts: "che" and "che-workspace".
// Also this method binds "edit" default k8s clusterrole using rolebinding to "che" SA.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheSameNamespaceWithChe(deployContext *deploy.DeployContext) (reconcile.Result, error) {
	tests := r.tests

	// Create "che-workspace" service account.
	// Che workspace components use this service account.
	cheWorkspaceSA, err := deploy.SyncServiceAccountToCluster(deployContext, CheWorkspacesServiceAccount)
	if cheWorkspaceSA == nil {
		logrus.Infof("Waiting on service account '%s' to be created", CheWorkspacesServiceAccount)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{}, err
		}
	}

	// Create view role for "che-workspace" service account.
	// This role used by exec terminals, tasks, metric che-theia plugin and so on.
	viewRole, err := deploy.SyncViewRoleToCluster(deployContext)
	if viewRole == nil {
		logrus.Infof("Waiting on role '%s' to be created", deploy.ViewRoleName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{}, err
		}
	}

	cheWSViewRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, ViewRoleBindingName, CheWorkspacesServiceAccount, deploy.ViewRoleName, "Role")
	if cheWSViewRoleBinding == nil {
		logrus.Infof("Waiting on role binding '%s' to be created", ViewRoleBindingName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{}, err
		}
	}

	// Create exec role for "che-workspaces" service account.
	// This role used by exec terminals, tasks and so on.
	execRole, err := deploy.SyncExecRoleToCluster(deployContext)
	if execRole == nil {
		logrus.Infof("Waiting on role '%s' to be created", deploy.ExecRoleName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{}, err
		}
	}

	cheWSExecRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, ExecRoleBindingName, CheWorkspacesServiceAccount, deploy.ExecRoleName, "Role")
	if cheWSExecRoleBinding == nil {
		logrus.Infof("Waiting on role binding '%s' to be created", ExecRoleBindingName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{}, err
		}
	}

	// Bind "edit" cluster role for "che" service account.
	// che-operator doesn't create "edit" role. This role is pre-created on the cluster.
	// Warning: operator binds clusterrole using rolebinding(not clusterrolebinding).
	// That's why "che" service account has got permissions only in the one namespace!
	// So permissions are binding in "non-cluster" scope.
	cheRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, EditRoleBindingName, CheServiceAccountName, EditClusterRoleName, "ClusterRole")
	if cheRoleBinding == nil {
		logrus.Infof("Waiting on role binding '%s' to be created", EditRoleBindingName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// Create cluster roles and cluster role bindings for "che" service account.
// che-server uses "che" service account for creation new workspaces and workspace components.
// Operator will create two cluster roles:
// - "<workspace-namespace/project-name>-clusterrole-manage-namespaces" - cluster role to mange namespace(for Kubernetes platform)
//    or project(for Openshift platform) for new workspace.
// - "<workspace-namespace/project-name>-clusterrole-workspaces" - cluster role to create and manage k8s objects required for
//    workspace components.
// Notice: After permission delegation che-server will create service account "che-workspace" ITSELF with
//         "exec" and "view" roles for each new workspace.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(instance *orgv1.CheCluster, deployContext *deploy.DeployContext) (reconcile.Result, error) {
	tests := r.tests

	cheCreateNamespacesName := fmt.Sprintf(CheWorkspacesNamespaceClusterRoleNameTemplate, instance.Namespace)
	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-manage-namespaces" to manage namespace/projects for Che workspaces.
	provisioned, err := deploy.SyncClusterRoleToCheCluster(deployContext, cheCreateNamespacesName, getCheWorkspacesNamespacePolicy())
	if !provisioned {
		logrus.Infof("Waiting on clusterrole '%s' to be created", cheCreateNamespacesName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	cheCreateNamespacesRolebinding, err := deploy.SyncClusterRoleBindingToCluster(deployContext, cheCreateNamespacesName, CheServiceAccountName, cheCreateNamespacesName)
	if cheCreateNamespacesRolebinding == nil {
		logrus.Infof("Waiting on clusterrolebinding '%s' to be created", cheCreateNamespacesName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheManageNamespacesName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, instance.Namespace)
	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-workspaces" to create k8s components for Che workspaces.
	provisioned, err = deploy.SyncClusterRoleToCheCluster(deployContext, cheManageNamespacesName, getCheWorkspacesPolicy())
	if !provisioned {
		logrus.Infof("Waiting on clusterrole '%s' to be created", cheManageNamespacesName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	cheManageNamespacesRolebinding, err := deploy.SyncClusterRoleBindingToCluster(deployContext, cheManageNamespacesName, CheServiceAccountName, cheManageNamespacesName)
	if cheManageNamespacesRolebinding == nil {
		logrus.Infof("Waiting on clusterrolebinding '%s' to be created", cheManageNamespacesName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	return reconcile.Result{}, nil
}

// DeleteWorkspacesInSameNamespaceWithChePermissions - removes workspaces in same namespace with Che role and rolebindings.
func (r *ReconcileChe) DeleteWorkspacesInSameNamespaceWithChePermissions(instance *orgv1.CheCluster, cli client.Client) error {
	logrus.Info("Delete workspaces in the same namespace with Che permissions.")

	if err := deploy.DeleteRole(deploy.ExecRoleName, instance.Namespace, cli); err != nil {
		return err
	}
	if err := deploy.DeleteRoleBinding(ExecRoleBindingName, instance.Namespace, cli); err != nil {
		return err
	}

	if err := deploy.DeleteRole(deploy.ViewRoleName, instance.Namespace, cli); err != nil {
		return err
	}
	if err := deploy.DeleteRoleBinding(ExecRoleBindingName, instance.Namespace, cli); err != nil {
		return err
	}

	if err := deploy.DeleteRoleBinding(EditRoleBindingName, instance.Namespace, cli); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileChe) reconcileWorkspacePermissionsFinalizer(instance *orgv1.CheCluster, deployContext *deploy.DeployContext) error {
	tests := r.tests
	if !util.IsOAuthEnabled(instance) && !util.IsWorkspaceInSameNamespaceWithChe(instance) {
		if err := r.DeleteWorkspacesInSameNamespaceWithChePermissions(instance, deployContext.ClusterAPI.Client); err != nil {
			return err
		}
		if !tests {
			if err := r.ReconcileClusterPermissionsFinalizer(instance); err != nil {
				logrus.Errorf("unable to add workspace permissions finalizers to the CR, cause %s", err.Error())
				return err
			}
		}
	} else {
		if !tests {
			// check if permissions exist to remove clusterrole & clusterrolebinding
			deniedPolicies, err := r.permissionChecker.GetNotPermittedPolicyRules(getDeleteClusterPermissionsPolicy(), "")
			if err != nil {
				return err
			}
			if len(deniedPolicies) == 0 {
				if err := r.RemoveWorkspaceClusterPermissions(instance); err != nil {
					logrus.Errorf("workspace permissions finalizers was not removed from CR, cause %s", err.Error())
					return err
				}
			}
		}
	}
	return nil
}

func getDeleteClusterPermissionsPolicy() []rbac.PolicyRule {
	return []rbac.PolicyRule{
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"clusterroles"},
			Verbs:     []string{"delete"},
		},
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"clusterrolebindings"},
			Verbs:     []string{"delete"},
		},
	}
}

func getCheWorkspacesNamespacePolicy() []rbac.PolicyRule {
	return []rbac.PolicyRule{
		{
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projectrequests"},
			Verbs:     []string{"create", "update"},
		},
		{
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projects"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update"},
		},
	}
}

func getCheWorkspacesPolicy() []rbac.PolicyRule {
	return []rbac.PolicyRule{
		{
			APIGroups: []string{"authorization.openshift.io", "rbac.authorization.k8s.io"},
			Resources: []string{"roles"},
			Verbs:     []string{"get", "create"},
		},
		{
			APIGroups: []string{"authorization.openshift.io", "rbac.authorization.k8s.io"},
			Resources: []string{"rolebindings"},
			Verbs:     []string{"get", "update", "create"},
		},
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
			APIGroups: []string{"apps"},
			Resources: []string{"secrets"},
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
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "create", "list", "watch", "patch", "delete"},
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
			APIGroups: []string{"route.openshift.io"},
			Resources: []string{"routes"},
			Verbs:     []string{"list", "create", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"watch"},
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
	}
}
