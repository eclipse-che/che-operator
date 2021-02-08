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
	"context"
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

func (r *ReconcileChe) ReconcileCheWorkspacesClusterPermissionsFinalizer(instance *orgv1.CheCluster) (err error) {
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(instance.ObjectMeta.Finalizers, cheWorkspacesClusterPermissionsFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, cheWorkspacesClusterPermissionsFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				return err
			}
		}
	} else {
		r.RemoveCheWorkspacesClusterPermissions(instance)
	}
	return nil
}

func (r *ReconcileChe) RemoveCheWorkspacesClusterPermissions(instance *orgv1.CheCluster) (err error) {
	if util.ContainsString(instance.ObjectMeta.Finalizers, cheWorkspacesClusterPermissionsFinalizerName) {
		logrus.Infof("Removing '%s'", cheWorkspacesClusterPermissionsFinalizerName)

		cheWorkspacesNamespaceClusterRoleName := fmt.Sprintf(CheWorkspacesNamespaceClusterRoleNameTemplate, instance.Namespace)
		cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, instance.Namespace)

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

		instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, cheWorkspacesClusterPermissionsFinalizerName)
		if err := r.client.Update(context.Background(), instance); err != nil {
			return err
		}
	}

	return nil
}
