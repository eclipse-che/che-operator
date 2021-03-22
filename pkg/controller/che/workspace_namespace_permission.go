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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
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
// - "<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" - cluster role to mange namespace(for Kubernetes platform)
//    or project(for Openshift platform) for new workspace.
// - "<workspace-namespace/project-name>-cheworkspaces-clusterrole" - cluster role to create and manage k8s objects required for
//    workspace components.
// Notice: After permission delegation che-server will create service account "che-workspace" ITSELF with
//         "exec" and "view" roles for each new workspace.
func (r *ReconcileChe) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(deployContext *deploy.DeployContext) (reconcile.Result, error) {
	tests := r.tests

	сheWorkspacesNamespaceClusterRoleName := fmt.Sprintf(CheWorkspacesNamespaceClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheWorkspacesNamespaceClusterRoleBindingName := сheWorkspacesNamespaceClusterRoleName
	// Create clusterrole "<workspace-namespace/project-name>-clusterrole-manage-namespaces" to manage namespace/projects for Che workspaces.
	provisioned, err := deploy.SyncClusterRoleToCluster(deployContext, сheWorkspacesNamespaceClusterRoleName, getCheWorkspacesNamespacePolicy())
	if !provisioned {
		logrus.Infof("Waiting on clusterrole '%s' to be created", сheWorkspacesNamespaceClusterRoleName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	done, err := deploy.SyncClusterRoleBindingToCluster(deployContext, сheWorkspacesNamespaceClusterRoleBindingName, CheServiceAccountName, сheWorkspacesNamespaceClusterRoleName)
	if !done {
		if !tests {
			logrus.Infof("Waiting on clusterrolebinding '%s' to be created", сheWorkspacesNamespaceClusterRoleBindingName)
			if err != nil {
				logrus.Error(err)
			}
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	сheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
	сheWorkspacesClusterRoleBindingName := сheWorkspacesClusterRoleName
	// Create clusterrole "<workspace-namespace/project-name>-cheworkspaces-namespaces-clusterrole" to create k8s components for Che workspaces.
	provisioned, err = deploy.SyncClusterRoleToCluster(deployContext, сheWorkspacesClusterRoleName, getCheWorkspacesPolicy())
	if !provisioned {
		logrus.Infof("Waiting on clusterrole '%s' to be created", сheWorkspacesClusterRoleName)
		if err != nil {
			logrus.Error(err)
		}
		if !tests {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	done, err = deploy.SyncClusterRoleBindingToCluster(deployContext, сheWorkspacesClusterRoleBindingName, CheServiceAccountName, сheWorkspacesClusterRoleName)
	if !done {
		if !tests {
			logrus.Infof("Waiting on clusterrolebinding '%s' to be created", сheWorkspacesClusterRoleBindingName)
			if err != nil {
				logrus.Error(err)
			}
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}
	return reconcile.Result{}, nil
}

// DeleteWorkspacesInSameNamespaceWithChePermissions - removes workspaces in same namespace with Che role and rolebindings.
func (r *ReconcileChe) DeleteWorkspacesInSameNamespaceWithChePermissions(instance *orgv1.CheCluster, cli client.Client) error {

	if err := deploy.DeleteRole(deploy.ExecRoleName, instance.Namespace, cli); err != nil {
		return err
	}
	if err := deploy.DeleteRoleBinding(ExecRoleBindingName, instance.Namespace, cli); err != nil {
		return err
	}

	if err := deploy.DeleteRole(deploy.ViewRoleName, instance.Namespace, cli); err != nil {
		return err
	}
	if err := deploy.DeleteRoleBinding(ViewRoleBindingName, instance.Namespace, cli); err != nil {
		return err
	}

	if err := deploy.DeleteRoleBinding(EditRoleBindingName, instance.Namespace, cli); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileChe) reconcileWorkspacePermissionsFinalizer(deployContext *deploy.DeployContext) error {

	if !util.IsOAuthEnabled(deployContext.CheCluster) {
		if util.IsWorkspaceInSameNamespaceWithChe(deployContext.CheCluster) {
			// Delete workspaces cluster permission set and finalizer from CR if deletion timestamp is not 0.
			if err := r.RemoveCheWorkspacesClusterPermissions(deployContext); err != nil {
				logrus.Errorf("workspace permissions finalizers was not removed from CR, cause %s", err.Error())
				return err
			}
		} else {
			// Delete permission set for configuration "same namespace for Che and workspaces".
			if err := r.DeleteWorkspacesInSameNamespaceWithChePermissions(deployContext.CheCluster, deployContext.ClusterAPI.Client); err != nil {
				logrus.Errorf("unable to delete workspaces in same namespace permission set, cause %s", err.Error())
				return err
			}
			// Add workspaces cluster permission finalizer to the CR if deletion timestamp is 0.
			// Or delete workspaces cluster permission set and finalizer from CR if deletion timestamp is not 0.
			if err := r.ReconcileCheWorkspacesClusterPermissionsFinalizer(deployContext); err != nil {
				logrus.Errorf("unable to add workspace permissions finalizers to the CR, cause %s", err.Error())
				return err
			}
		}
	} else {
		// Delete workspaces cluster permission set and finalizer from CR if deletion timestamp is not 0.
		if err := r.RemoveCheWorkspacesClusterPermissions(deployContext); err != nil {
			logrus.Errorf("workspace permissions finalizers was not removed from CR, cause %s", err.Error())
			return err
		}
	}

	return nil
}

func getCheWorkspacesNamespacePolicy() []rbac.PolicyRule {
	k8sPolicies := []rbac.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update"},
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
			Verbs:     []string{"get"},
		},
	}

	if util.IsOpenShift {
		return append(k8sPolicies, openshiftPolicies...)
	}
	return k8sPolicies
}

func getCheWorkspacesPolicy() []rbac.PolicyRule {
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
