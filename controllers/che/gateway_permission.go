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

package che

import (
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/util"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	CheGatewayClusterPermissionsFinalizerName = "cheGateway.clusterpermissions.finalizers.che.eclipse.org"
)

func (r *CheClusterReconciler) reconcileGatewayPermissions(deployContext *deploy.DeployContext) (bool, error) {
	if util.IsOpenShift && deployContext.CheCluster.IsNativeUserModeEnabled() {
		name := gatewayPermisisonsName(deployContext.CheCluster)
		if _, err := deploy.SyncClusterRoleToCluster(deployContext, name, getGatewayClusterRoleRules()); err != nil {
			return false, err
		}

		if _, err := deploy.SyncClusterRoleBindingToCluster(deployContext, name, gateway.GatewayServiceName, name); err != nil {
			return false, err
		}

		if err := deploy.AppendFinalizer(deployContext, CheGatewayClusterPermissionsFinalizerName); err != nil {
			return false, err
		}
	} else {
		return deleteGatewayPermissions(deployContext)
	}

	return true, nil
}

func (r *CheClusterReconciler) reconcileGatewayPermissionsFinalizers(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return deleteGatewayPermissions(deployContext)
	}

	return true, nil
}

func deleteGatewayPermissions(deployContext *deploy.DeployContext) (bool, error) {
	name := gatewayPermisisonsName(deployContext.CheCluster)
	if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRole{}); !done {
		return false, err
	}

	if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}); !done {
		return false, err
	}

	if err := deploy.DeleteFinalizer(deployContext, CheGatewayClusterPermissionsFinalizerName); err != nil {
		return false, err
	}

	return true, nil
}

func gatewayPermisisonsName(instance *orgv1.CheCluster) string {
	return instance.Namespace + "-" + gateway.GatewayServiceName
}

func getGatewayClusterRoleRules() []rbac.PolicyRule {
	return []rbac.PolicyRule{
		{
			Verbs:     []string{"create"},
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"tokenreviews"},
		},
		{
			Verbs:     []string{"create"},
			APIGroups: []string{"authorization.k8s.io"},
			Resources: []string{"subjectaccessreviews"},
		},
	}
}
