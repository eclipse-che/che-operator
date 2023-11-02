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

package rbac

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CheGatewayClusterPermissionsFinalizerName = "cheGateway.clusterpermissions.finalizers.che.eclipse.org"
)

type GatewayPermissionsReconciler struct {
	deploy.Reconcilable
}

func NewGatewayPermissionsReconciler() *GatewayPermissionsReconciler {
	return &GatewayPermissionsReconciler{}
}

func (gp *GatewayPermissionsReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	name := gp.gatewayPermissionsName(ctx.CheCluster)
	if done, err := deploy.SyncClusterRoleToCluster(ctx, name, gp.getGatewayClusterRoleRules()); !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	if done, err := deploy.SyncClusterRoleBindingToCluster(ctx, name, gateway.GatewayServiceName, name); !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	if err := deploy.AppendFinalizer(ctx, CheGatewayClusterPermissionsFinalizerName); err != nil {
		return reconcile.Result{Requeue: true}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (gp *GatewayPermissionsReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	if _, err := gp.deleteGatewayPermissions(ctx); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}
	return true
}

func (gp *GatewayPermissionsReconciler) deleteGatewayPermissions(deployContext *chetypes.DeployContext) (bool, error) {
	name := gp.gatewayPermissionsName(deployContext.CheCluster)
	if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{}); !done {
		return false, err
	}

	if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbacv1.ClusterRole{}); !done {
		return false, err
	}

	if err := deploy.DeleteFinalizer(deployContext, CheGatewayClusterPermissionsFinalizerName); err != nil {
		return false, err
	}

	return true, nil
}

func (gp *GatewayPermissionsReconciler) gatewayPermissionsName(instance *chev2.CheCluster) string {
	return instance.Namespace + "-" + gateway.GatewayServiceName
}

func (gp *GatewayPermissionsReconciler) getGatewayClusterRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
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
