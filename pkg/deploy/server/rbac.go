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

package server

import (
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	util "github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

// syncPermissions handles spec.components.cheServer.clusterRoles custom CRBs for the che SA.
func (s *CheServerReconciler) syncPermissions(ctx *chetypes.DeployContext) (bool, error) {
	for _, cheClusterRole := range ctx.CheCluster.Spec.Components.CheServer.ClusterRoles {
		cheClusterRole := strings.TrimSpace(cheClusterRole)
		if cheClusterRole != "" {
			if done, err := deploy.SyncClusterRoleBindingToCluster(ctx, cheClusterRole, constants.DefaultCheServiceAccountName, cheClusterRole); !done {
				return false, err
			}

			finalizer := s.getCRBFinalizerName(cheClusterRole)
			if err := deploy.AppendFinalizer(ctx, finalizer); err != nil {
				return false, err
			}
		}
	}

	// Delete abandoned CRBs
	for _, finalizer := range ctx.CheCluster.Finalizers {
		if strings.HasSuffix(finalizer, cheCRBFinalizerSuffix) {
			cheClusterRole := strings.TrimSuffix(finalizer, cheCRBFinalizerSuffix)
			if !util.Contains(ctx.CheCluster.Spec.Components.CheServer.ClusterRoles, cheClusterRole) {
				if done, err := deploy.Delete(ctx, types.NamespacedName{Name: cheClusterRole}, &rbacv1.ClusterRoleBinding{}); !done {
					return false, err
				}

				if err := deploy.DeleteFinalizer(ctx, finalizer); err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

func (s *CheServerReconciler) deletePermissions(ctx *chetypes.DeployContext) bool {
	done := true

	for _, name := range ctx.CheCluster.Spec.Components.CheServer.ClusterRoles {
		name := strings.TrimSpace(name)
		if name != "" {
			if _, err := deploy.Delete(ctx, types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{}); err != nil {
				done = false
				logrus.Errorf("Failed to delete ClusterRoleBinding '%s', cause: %v", name, err)
			}

			// Removes any legacy CRB https://github.com/eclipse/che/issues/19506
			legacyName := ctx.CheCluster.Namespace + "-" + constants.DefaultCheServiceAccountName + "-" + name
			if _, err := deploy.Delete(ctx, types.NamespacedName{Name: legacyName}, &rbacv1.ClusterRoleBinding{}); err != nil {
				done = false
				logrus.Errorf("Failed to delete ClusterRoleBinding '%s', cause: %v", legacyName, err)
			}
		}
	}

	return done
}
