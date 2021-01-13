//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"strings"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (r *ReconcileChe) ReconsileClusterPermissionsFinalizer(instance *orgv1.CheCluster) (err error) {
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
		logrus.Infof("Removing Cluster permissions finalizer on %s CR", instance.Name)

		cheCreateNamespacesName := fmt.Sprintf(CheCreateNamespaces, instance.Namespace)
		cheManageNamespacesName := fmt.Sprintf(CheManageNamespaces, instance.Namespace)
		
		if err := r.removeClusterRoleBinding(cheCreateNamespacesName, instance.Name); err != nil {
			return err
		}
		if err := r.removeClusterRole(cheCreateNamespacesName, instance.Name); err != nil {
			return err
		}
		if err := r.removeClusterRoleBinding(cheManageNamespacesName, instance.Name); err != nil {
			return err
		}
		if err := r.removeClusterRole(cheManageNamespacesName, instance.Name); err != nil {
			return err
		}

		instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, clusterPermissionsFinalizerName)
		finalizers := "[" + strings.Join(instance.ObjectMeta.Finalizers, ",") + "]"
		logrus.Infof("===========Finalizers %s================", finalizers)
		if err := r.client.Update(context.Background(), instance); err != nil {
			logrus.Errorf("============Sad CR was not updated, cause %s", err.Error())
			return err
		}
	}

	return nil
}

func (r *ReconcileChe) removeClusterRole(clusterRoleName string, cheClusterName string) error {
	logrus.Infof("Custom resource %s is being deleted. Deleting Cluster role %s.", cheClusterName, clusterRoleName)

	clusterRole, err := deploy.GetClusterRole(clusterRoleName, r.nonCachedClient)
	if err == nil && clusterRole != nil {
		// todo remove this log when pr is well tested
		logrus.Info("Cluster role is %v", clusterRole)
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
		// todo remove this log when pr is well tested
		logrus.Info("cluster role binding is %v", clusterRoleBinding)
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
