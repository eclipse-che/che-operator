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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
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

		if err := deploy.DeleteClusterRole(cheWorkspacesNamespaceClusterRoleName, r.nonCachedClient); err != nil {
			return err
		}
		if err := deploy.DeleteClusterRoleBinding(cheWorkspacesNamespaceClusterRoleName, r.nonCachedClient); err != nil {
			return err
		}
		if err := deploy.DeleteClusterRole(cheWorkspacesClusterRoleName, r.nonCachedClient); err != nil {
			return err
		}
		if err := deploy.DeleteClusterRoleBinding(cheWorkspacesClusterRoleName, r.nonCachedClient); err != nil {
			return err
		}

		return deploy.DeleteFinalizer(deployContext, cheWorkspacesClusterPermissionsFinalizerName)
	}

	return nil
}
