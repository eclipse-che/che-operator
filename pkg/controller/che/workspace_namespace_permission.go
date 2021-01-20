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
	EditRole                     = "edit"
	EditRoleBinding              = "che"
	ViewRoleBinding              = "che-workspace-view"
	ExecRoleBinding              = "che-workspace-exec"
	CheWorkspacesServiceAccount  = "che-workspace"
	CheCreateNamespacesTemplate  = "%s-clusterrole-create-namespaces"
	CheManageNamespacesTempalate = "%s-clusterrole-manage-namespaces"
	CheRoleBindingName           = "che"
)

func (r *ReconcileChe) delegateWorkspacePermissionsInTheSameNamespaceWithChe(deployContext *deploy.DeployContext) (reconcile.Result, error) {
	tests := r.tests
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

	// create view role for CheCluster server and workspaces
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

	// create exec role for CheCluster server and workspaces
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

func (r *ReconcileChe) delegateWorkspacePermissionsInTheDifferNamespaceThanChe(instance *orgv1.CheCluster, deployContext *deploy.DeployContext) (reconcile.Result, error) {
	tests := r.tests
	cheManageNamespacesName := fmt.Sprintf(CheManageNamespacesTempalate, instance.Namespace)
	cheManageNamespacesClusterRole, err := deploy.SyncClusterRoleToCheCluster(deployContext, cheManageNamespacesName, getCheManageNamespacesPolicy())
	if cheManageNamespacesClusterRole == nil {
		logrus.Infof("Waiting on clusterrole '%s' to be created", cheManageNamespacesName)
		if err != nil {
			logrus.Error(err)
		}
		return reconcile.Result{RequeueAfter: time.Second}, err
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

	cheCreateNamespacesName := fmt.Sprintf(CheCreateNamespacesTemplate, instance.Namespace)
	cheCreateNamespacesRole, err := deploy.SyncClusterRoleToCheCluster(deployContext, cheCreateNamespacesName, getCheCreateNamespacesPolicy())
	if cheCreateNamespacesRole == nil {
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
	return reconcile.Result{}, nil
}

func (r *ReconcileChe) reconsileWorkspacePermissionsFinalizer(instance *orgv1.CheCluster, deployContext *deploy.DeployContext) error {
	// logrus.Infof("====================================Test!!!! deletion timestamp is zero %t===================================", instance.ObjectMeta.DeletionTimestamp.IsZero())
	tests := r.tests
	if !util.IsOAuthEnabled(instance) && !util.IsWorkspacesInTheSameNamespaceWithChe(instance) {
		// logrus.Info("=========Reconsile finalizers!!!!====================")
		if !tests {
			if err := r.ReconsileClusterPermissionsFinalizer(instance); err != nil {
				return err
			}
		}
	} else {
		// logrus.Info("=============Remove workspace permissions===========")
		if !tests {
			deniedPolicies, err := getNotPermittedPolicyRules(getDeleteClusterRoleAndBindingPolicy(), "")
			if err != nil {
				return err
			}
			if len(deniedPolicies) == 0 {
				if err := r.RemoveWorkspaceClusterPermissions(instance); err != nil {
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

func getCheManageNamespacesPolicy() []rbac.PolicyRule {
	return []rbac.PolicyRule{
		{
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projectrequests"},
			Verbs:     []string{"create", "update"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update"},
		},
	}
}

func getCheCreateNamespacesPolicy() []rbac.PolicyRule {
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
			APIGroups: []string{"project.openshift.io"},
			Resources: []string{"projects"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get"},
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
