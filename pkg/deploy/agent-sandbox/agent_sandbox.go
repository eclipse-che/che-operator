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

package agentsandbox

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AgentSandboxReconciler struct {
	reconciler.Reconcilable
}

func NewAgentSandboxReconciler() *AgentSandboxReconciler {
	return &AgentSandboxReconciler{}
}

func (r *AgentSandboxReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if cheCtx.CheCluster.IsAgentSandboxEnabled() {
		if err := r.sync(cheCtx); err != nil {
			return reconcile.Result{}, false, err
		}
	} else {
		if err := r.delete(cheCtx); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (r *AgentSandboxReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	return r.delete(cheCtx)
}

func (r *AgentSandboxReconciler) sync(cheCtx *chetypes.CheContext) error {
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   GetUserClusterRoleName(),
			Labels: deploy.GetLabels(constants.AgentSandboxComponentName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"agents.x-k8s.io",
				},
				Resources: []string{
					"sandboxes",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"create",
					"update",
					"patch",
					"delete",
				},
			},
		},
	}

	if err := cheCtx.ClusterAPI.ClientWrapper.Sync(
		cheCtx.Context,
		clusterRole,
		&clientwrapper.SyncOptions{DiffOpts: diffs.ClusterRole},
	); err != nil {
		return fmt.Errorf("failed to sync ClusterRole %s: %w", clusterRole.Name, err)
	}

	return nil
}

func (r *AgentSandboxReconciler) delete(cheCtx *chetypes.CheContext) error {
	clusterRoleKey := types.NamespacedName{Name: GetUserClusterRoleName()}

	if err := cheCtx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		cheCtx.Context,
		clusterRoleKey,
		&rbacv1.ClusterRole{},
	); err != nil {
		return fmt.Errorf("failed to delete ClusterRole %s: %w", clusterRoleKey.String(), err)
	}

	return nil
}
