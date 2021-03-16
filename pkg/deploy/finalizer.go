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
package deploy

import (
	"context"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AppendFinalizer(deployContext *DeployContext, finalizer string) error {
	if !util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		logrus.Infof("Adding finalizer: %s", finalizer)
		deployContext.CheCluster.ObjectMeta.Finalizers = append(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer)
		for {
			err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
			if err == nil || !errors.IsConflict(err) {
				return err
			}

			err = util.ReloadCheCluster(deployContext.ClusterAPI.Client, deployContext.CheCluster)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteFinalizer(deployContext *DeployContext, finalizer string) error {
	logrus.Infof("Deleting finalizer: %s", finalizer)
	deployContext.CheCluster.ObjectMeta.Finalizers = util.DoRemoveString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer)
	for {
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
		if err == nil || !errors.IsConflict(err) {
			return err
		}

		err = util.ReloadCheCluster(deployContext.ClusterAPI.Client, deployContext.CheCluster)
		if err != nil {
			return err
		}
	}
}

func DeleteObjectAndFinalizer(deployContext *DeployContext, key client.ObjectKey, blueprint metav1.Object, finalizer string) error {
	_, err := Delete(deployContext, key, blueprint)
	if err != nil {
		// failed to delete, shouldn't us prevent from removing finalizer
		logrus.Error(err)
	}

	return DeleteFinalizer(deployContext, finalizer)
}

func GetFinalizerName(prefix string) string {
	return prefix + ".finalizers.che.eclipse.org"
}
