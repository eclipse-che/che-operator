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
	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AgentSandboxReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *AgentSandboxReconciler {
	return &AgentSandboxReconciler{}
}

func (r *AgentSandboxReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if cheCtx.CheCluster.IsAgentSandboxEnabled() {
		if err := r.syncAll(cheCtx); err != nil {
			return reconcile.Result{}, false, err
		}
	} else {
		if err := r.deleteAll(cheCtx); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (r *AgentSandboxReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	return r.deleteAll(cheCtx)
}

func (r *AgentSandboxReconciler) syncAll(cheCtx *chetypes.CheContext) error {
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   GetUserClusterRoleName(),
			Labels: chelabels.GetStandardLabels(constants.AgentSandboxComponentName),
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
	return resources.ClusterRole.Sync(cheCtx, clusterRole)
}

func (r *AgentSandboxReconciler) deleteAll(cheCtx *chetypes.CheContext) error {
	return resources.ClusterRole.Delete(cheCtx, GetUserClusterRoleName())
}

func GetUserClusterRoleName() string {
	return defaults.GetCheFlavor() + "-user-agent-sandbox"
}

func GetUserRoleBindingName() string {
	return defaults.GetCheFlavor() + "-user-agent-sandbox"
}
