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
	"k8s.io/apimachinery/pkg/api/errors"
)

func (r *ReconcileChe) ReconcileClusterPermissionsFinalizer(instance *orgv1.CheCluster) (err error) {
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(instance.ObjectMeta.Finalizers, clusterPermissionsFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, clusterPermissionsFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				return err
			}
		}
	} else {
		r.RemoveWorkspaceClusterPermissions(instance)
	}
	return nil
}

func (r *ReconcileChe) RemoveWorkspaceClusterPermissions(instance *orgv1.CheCluster) (err error) {
	if util.ContainsString(instance.ObjectMeta.Finalizers, clusterPermissionsFinalizerName) {
		logrus.Info("Removing workspace cluster permissions finalizer")

		cheWorkspacesNamespaceClusterRoleName := fmt.Sprintf(CheWorkspacesNamespaceClusterRoleNameTemplate, instance.Namespace)
		cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, instance.Namespace)

		if err := r.removeClusterRoleBinding(cheWorkspacesNamespaceClusterRoleName, instance.Name); err != nil {
			return err
		}
		if err := r.removeClusterRole(cheWorkspacesNamespaceClusterRoleName, instance.Name); err != nil {
			return err
		}
		if err := r.removeClusterRoleBinding(cheWorkspacesClusterRoleName, instance.Name); err != nil {
			return err
		}
		if err := r.removeClusterRole(cheWorkspacesClusterRoleName, instance.Name); err != nil {
			return err
		}

		instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, clusterPermissionsFinalizerName)
		if err := r.client.Update(context.Background(), instance); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileChe) removeClusterRole(clusterRoleName string, cheClusterName string) error {
	logrus.Infof("Custom resource is being deleted. Deleting Cluster role %s.", clusterRoleName)

	clusterRole, err := deploy.GetClusterRole(clusterRoleName, r.nonCachedClient)
	if err == nil && clusterRole != nil {
		if err := r.nonCachedClient.Delete(context.TODO(), clusterRole); err != nil {
			logrus.Errorf("Failed to delete %s clusterRole: %s", clusterRoleName, err.Error())
			return err
		}
	} else if !errors.IsNotFound(err) {
		logrus.Errorf("Failed to get %s clusterRole: %s", clusterRoleName, err)
		return err
	}

	return nil
}

func (r *ReconcileChe) removeClusterRoleBinding(clusterRoleBindingName string, cheClusterName string) error {
	logrus.Infof("Custom resource %s is being deleted. Deleting Cluster rolebinding %s.", cheClusterName, clusterRoleBindingName)

	clusterRoleBinding, err := deploy.GetClusterRoleBiding(clusterRoleBindingName, r.nonCachedClient)
	if err == nil && clusterRoleBinding != nil {
		if err := r.nonCachedClient.Delete(context.TODO(), clusterRoleBinding); err != nil {
			logrus.Errorf("Failed to delete %s clusterRoleBinding: %s", clusterRoleBindingName, err.Error())
			return err
		}
	} else if !errors.IsNotFound(err) {
		logrus.Errorf("Failed to get %s clusterRoleBinding: %s", clusterRoleBindingName, err)
		return err
	}

	return nil
}
