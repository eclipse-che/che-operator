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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// EditRole - default "edit" cluster role. This role is pre-created on the cluster.
	// See more: https://kubernetes.io/blog/2017/10/using-rbac-generally-available-18/#granting-access-to-users
	EditRole = "edit"
	// EditRoleBinding - "edit" rolebinding for che-server.
	EditRoleBinding = "che"
	// CheWorkspacesServiceAccount - service account created for Che workspaces.
	CheWorkspacesServiceAccount = "che-workspace"
	// ViewRoleBinding - "view" role for "che-workspace" service account.
	ViewRoleBinding = "che-workspace-view"
	// ExecRoleBinding - "exec" role for "che-workspace" service account.
	ExecRoleBinding = "che-workspace-exec"
	// CheCreateNamespacesTemplate - create namespaces "cluster role" and "clusterrolebinding" template name
	CheCreateNamespacesTemplate = "%s-clusterrole-create-namespaces"
	// CheManageNamespacesTempalate - manage namespaces "cluster role" and "clusterrolebinding" template name
	CheManageNamespacesTempalate = "%s-clusterrole-manage-namespaces"
)

// delegateWorkspacePermissionsInTheSameNamespaceWithChe - creates "che-workspace" service account and
// delegates "che-oparator" SA permissions to the service account "che" and service account for Che workspaces - "che-workspace".
// Also this method binds "edit" default k8s clusterrole using rolebinding to "che" SA.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheSameNamespaceWithChe(deployContext *deploy.DeployContext) (reconcile.Result, error) {
	logrus.Info("Configure permissions for Che workspaces. Workspaces will be executed in the same namespace with Che")
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
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// Create view role for "che-workspace" service account.
	// This role used by exec terminals, tasks, metric che-theia plugin and so on.
	viewRole, err := deploy.SyncViewRoleToCluster(deployContext)
	if viewRole == nil {
		logrus.Infof("Waiting on role '%s' to be created", deploy.ViewRole)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWSViewRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, ViewRoleBinding, CheWorkspacesServiceAccount, deploy.ViewRole, "Role")
	if cheWSViewRoleBinding == nil {
		logrus.Infof("Waiting on role binding '%s' to be created", ViewRoleBinding)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// Create exec role for "che-workspaces" service account.
	// This role used by exec terminals, tasks and so on.
	execRole, err := deploy.SyncExecRoleToCluster(deployContext)
	if execRole == nil {
		logrus.Infof("Waiting on role '%s' to be created", deploy.ExecRole)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	cheWSExecRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, ExecRoleBinding, CheWorkspacesServiceAccount, deploy.ExecRole, "Role")
	if cheWSExecRoleBinding == nil {
		logrus.Infof("Waiting on role binding '%s' to be created", ExecRoleBinding)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// Bind "edit" cluster role for "che" service account.
	// che-operator doesn't create "edit" role. This role is pre-created on the cluster.
	// Warning: operator binds clusterrole using rolebinding(not clusterrolebinding).
	// That's why "che" service account has got permissions only in the one namespace!
	// So permissions are binding in "non-cluster" scope.
	cheRoleBinding, err := deploy.SyncRoleBindingToCluster(deployContext, EditRoleBinding, CheServiceAccountName, EditRole, "ClusterRole")
	if cheRoleBinding == nil {
		logrus.Infof("Waiting on role binding '%s' to be created", EditRoleBinding)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	return reconcile.Result{}, nil
}

// Create cluster roles and cluster role bindings for "che" service account.
// che-server uses "che" service account for creation new workspaces and workspace components.
// Operator will create two cluster roles:
// - "<workspace-namespace/project-name>-clusterrole-create-namespaces" - cluster role to create namespace(for Kubernetes platform)
//    or project(for Openshift platform) for new workspace.
// - "<workspace-namespace/project-name>-clusterrole-manage-namespaces" - cluster role to create and manage k8s objects required for
//    workspace components.
// Notice: After permission delegation che-server will create service account "che-workspace" ITSELF with
//         "exec" and "view" roles for each new workspace.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(instance *orgv1.CheCluster, deployContext *deploy.DeployContext) (reconcile.Result, error) {
	logrus.Info("Configure permissions for Che workspaces. Workspaces will be executed in the differ namespace than Che namespace")
	tests := r.tests

	cheCreateNamespacesName := fmt.Sprintf(CheCreateNamespacesTemplate, instance.Namespace)
	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-create-namespaces" to create namespace/projects for Che workspaces.
	roleCNsynchronized, err := deploy.SyncClusterRoleToCheCluster(deployContext, cheCreateNamespacesName, getCheCreateNamespacesPolicy())
	if !roleCNsynchronized {
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

	cheManageNamespacesName := fmt.Sprintf(CheManageNamespacesTempalate, instance.Namespace)
	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-manage-namespaces" to create k8s components for Che workspaces.
	roleMNSynchronized, err := deploy.SyncClusterRoleToCheCluster(deployContext, cheManageNamespacesName, getCheManageNamespacesPolicy())
	if !roleMNSynchronized {
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

func (r *ReconcileChe) reconsileWorkspacePermissionsFinalizer(instance *orgv1.CheCluster, deployContext *deploy.DeployContext) error {
	tests := r.tests
	if !util.IsOAuthEnabled(instance) && !util.IsWorkspacesInTheSameNamespaceWithChe(instance) {
		if !tests {
			if err := r.ReconsileClusterPermissionsFinalizer(instance); err != nil {
				logrus.Errorf("unable to add workspace permissions finalizers to the CR, cause %s", err.Error())
				return err
			}
		}
	} else {
		if !tests {
			deniedPolicies, err := r.permissionChecker.GetNotPermittedPolicyRules(getDeleteClusterRoleAndBindingPolicy(), "")
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

func getDeleteClusterRoleAndBindingPolicy() []rbac.PolicyRule {
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

func getCheCreateNamespacesPolicy() []rbac.PolicyRule {
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

func getCheManageNamespacesPolicy() []rbac.PolicyRule {
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
