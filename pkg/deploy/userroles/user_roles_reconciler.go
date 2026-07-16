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

package userroles

import (
	"fmt"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	userRolesFinalizerName            = "cheUserRoles.rbac.finalizers.che.eclipse.org"
	userCommonPermissionsTemplateName = "%s-cheworkspaces-clusterrole"
	userDWPermissionsTemplateName     = "%s-cheworkspaces-devworkspace-clusterrole"
)

// GetDefaultUserClusterRoles returns the names of the ClusterRoles managed by UserRolesReconciler
// for the given namespace.
func GetDefaultUserClusterRoles(namespace string) []string {
	return []string{
		fmt.Sprintf(userCommonPermissionsTemplateName, namespace),
		fmt.Sprintf(userDWPermissionsTemplateName, namespace),
	}
}

// UserRolesReconciler reconciles workspace ClusterRoles and per-SA bindings.
type UserRolesReconciler struct {
	reconciler.Reconcilable
}

// NewUserRolesReconciler creates a new UserRolesReconciler.
func NewUserRolesReconciler() *UserRolesReconciler {
	return &UserRolesReconciler{}
}

func (ur *UserRolesReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	namespace := ctx.CheCluster.Namespace

	policies := map[string][]rbacv1.PolicyRule{
		fmt.Sprintf(userCommonPermissionsTemplateName, namespace): ur.getUserCommonPolicies(),
		fmt.Sprintf(userDWPermissionsTemplateName, namespace):     ur.getUserDevWorkspacePolicies(),
	}

	for name, policy := range policies {
		if done, err := deploy.SyncClusterRoleToCluster(ctx, name, policy); !done {
			return reconcile.Result{RequeueAfter: time.Second}, false, err
		}

		if done, err := deploy.SyncClusterRoleBindingToCluster(ctx, name, constants.DefaultCheServiceAccountName, name); !done {
			return reconcile.Result{RequeueAfter: time.Second}, false, err
		}
	}

	if err := deploy.AppendFinalizer(ctx, userRolesFinalizerName); err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (ur *UserRolesReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	namespace := ctx.CheCluster.Namespace

	names := []string{
		fmt.Sprintf(userCommonPermissionsTemplateName, namespace),
		fmt.Sprintf(userDWPermissionsTemplateName, namespace),
	}

	done := true

	for _, name := range names {
		if _, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{}); err != nil {
			done = false
			logrus.Errorf("Error deleting ClusterRoleBinding '%s': %v", name, err)
		}
		if _, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRole{}); err != nil {
			done = false
			logrus.Errorf("Error deleting ClusterRole '%s': %v", name, err)
		}
	}

	if !done {
		return false
	}

	if err := deploy.DeleteFinalizer(ctx, userRolesFinalizerName); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}

	return true
}

func (ur *UserRolesReconciler) getUserDevWorkspacePolicies() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{"workspace.devfile.io"},
			Resources: []string{"devworkspaces", "devworkspacetemplates"},
			Verbs:     []string{"get", "create", "delete", "list", "update", "patch", "watch"},
		},
	}
}

func (ur *UserRolesReconciler) getUserCommonPolicies() []rbacv1.PolicyRule {
	k8sPolicies := []rbacv1.PolicyRule{
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
			Resources: []string{"pods/portforward"},
			Verbs:     []string{"get", "list", "create"},
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
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch", "create", "delete"},
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

	if infrastructure.IsOpenShift() {
		openshiftPolicies := []rbacv1.PolicyRule{
			{
				APIGroups: []string{"route.openshift.io"},
				Resources: []string{"routes"},
				Verbs:     []string{"get", "list", "create", "delete"},
			},
			{
				APIGroups: []string{"project.openshift.io"},
				Resources: []string{"projects"},
				Verbs:     []string{"get"},
			},
		}
		return append(k8sPolicies, openshiftPolicies...)
	}
	return k8sPolicies
}
