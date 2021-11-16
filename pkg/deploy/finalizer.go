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
package deploy

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AppendFinalizer(deployContext *DeployContext, finalizer string) error {
	if !util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		for {
			deployContext.CheCluster.ObjectMeta.Finalizers = append(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer)
			err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
			if err == nil {
				logrus.Infof("Added finalizer: %s", finalizer)
				return nil
			} else if !errors.IsConflict(err) {
				return err
			}

			err = ReloadCheClusterCR(deployContext)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteFinalizer(deployContext *DeployContext, finalizer string) error {
	if util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		for {
			deployContext.CheCluster.ObjectMeta.Finalizers = util.DoRemoveString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer)
			err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
			if err == nil {
				logrus.Infof("Deleted finalizer: %s", finalizer)
				return nil
			} else if !errors.IsConflict(err) {
				return err
			}

			err = ReloadCheClusterCR(deployContext)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteObjectWithFinalizer(deployContext *DeployContext, key client.ObjectKey, objectMeta client.Object, finalizer string) error {
	_, err := Delete(deployContext, key, objectMeta)
	if err != nil {
		// failed to delete, shouldn't us prevent from removing finalizer
		logrus.Error(err)
	}

	return DeleteFinalizer(deployContext, finalizer)
}

func GetFinalizerName(prefix string) string {
	finalizer := prefix + ".finalizers.che.eclipse.org"
	diff := len(finalizer) - 63
	if diff > 0 {
		return finalizer[:len(finalizer)-diff]
	}
	return finalizer
}
