//
// Copyright (c) 2012-2020 Red Hat, Inc.
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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileChe) ReconcileCheWorkspacesClusterPermissionsFinalizer(deployContext *deploy.DeployContext) (err error) {
	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return deploy.AppendFinalizer(deployContext, cheWorkspacesClusterPermissionsFinalizerName)
	} else {
		r.RemoveCheWorkspacesClusterPermissions(deployContext)
	}
	return nil
}

func (r *ReconcileChe) RemoveCheWorkspacesClusterPermissions(deployContext *deploy.DeployContext) (err error) {
	if util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, cheWorkspacesClusterPermissionsFinalizerName) {
		logrus.Infof("Removing '%s'", cheWorkspacesClusterPermissionsFinalizerName)

		cheWorkspacesNamespaceClusterRoleName := fmt.Sprintf(CheWorkspacesNamespaceClusterRoleNameTemplate, deployContext.CheCluster.Namespace)
		cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, deployContext.CheCluster.Namespace)

		done, err := deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesNamespaceClusterRoleName}, &rbac.ClusterRole{})
		if !done || err != nil {
			return err
		}
		done, err = deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesNamespaceClusterRoleName}, &rbac.ClusterRoleBinding{})
		if !done || err != nil {
			return err
		}
		done, err = deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesClusterRoleName}, &rbac.ClusterRole{})
		if !done || err != nil {
			return err
		}
		done, err = deploy.Delete(deployContext, types.NamespacedName{Name: cheWorkspacesClusterRoleName}, &rbac.ClusterRoleBinding{})
		if !done || err != nil {
			return err
		}

		return deploy.DeleteFinalizer(deployContext, cheWorkspacesClusterPermissionsFinalizerName)
	}

	return nil
}
