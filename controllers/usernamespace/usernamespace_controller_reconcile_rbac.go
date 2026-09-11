//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package usernamespace

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	agentsandbox "github.com/eclipse-che/che-operator/pkg/deploy/agent-sandbox"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *CheUserNamespaceReconciler) reconcileAgentSandboxRbac(
	username string,
	targetNs string,
	ctx *chetypes.DeployContext,
) (bool, error) {
	if username == "" {
		logger.Info("AgentSandbox RBAC creation skipped because username is unknown", "namespace", targetNs)
		return true, nil
	}

	if !ctx.CheCluster.IsAgentSandboxEnabled() {
		roleBindingKey := types.NamespacedName{
			Name:      agentsandbox.GetUserRoleBindingName(),
			Namespace: targetNs,
		}

		if err := r.clientWrapper.DeleteByKeyIgnoreNotFound(
			ctx.Context,
			roleBindingKey,
			&rbacv1.RoleBinding{},
		); err != nil {
			return false, fmt.Errorf("failed to delete RoleBinding %s: %w", roleBindingKey.String(), err)
		}

		return true, nil
	}

	// Check ClusterRole existence before RoleBinding creation in order not to produce
	// unnecessary error in the console. When AgentSandbox integration enabled in the CheCluster,
	// the main controller requires some time to provision ClusterRole resource.
	// See pkg/deploy/agent-sandbox/agent_sandbox.go
	exists, err := r.clientWrapper.GetIgnoreNotFound(
		ctx.Context,
		types.NamespacedName{Name: agentsandbox.GetUserClusterRoleName()},
		&rbacv1.ClusterRole{},
	)
	if err != nil {
		return false, fmt.Errorf("failed to get ClusterRole %s: %w", agentsandbox.GetUserClusterRoleName(), err)
	}
	if !exists {
		logger.Info(
			fmt.Sprintf("AgentSandbox RBAC creation skipped because ClusterRole %s is missing", agentsandbox.GetUserClusterRoleName()),
			"namespace", targetNs,
		)
		return false, nil
	}

	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentsandbox.GetUserRoleBindingName(),
			Namespace: targetNs,
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: constants.AgentSandboxComponentName,
				constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     agentsandbox.GetUserClusterRoleName(),
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: "rbac.authorization.k8s.io",
				Name:     username,
			},
		},
	}

	if err := r.clientWrapper.Sync(
		ctx.Context,
		roleBinding,
		&k8sclient.SyncOptions{DiffOpts: diffs.RoleBinding},
	); err != nil {
		return false, fmt.Errorf("failed to sync RoleBinding %s/%s: %w", roleBinding.Namespace, roleBinding.Name, err)
	}

	return true, nil
}
