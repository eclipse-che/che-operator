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

package deploy

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CleanUpAllFinalizers(ctx *chetypes.DeployContext) error {
	ctx.CheCluster.Finalizers = []string{}
	return ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
}

func AppendFinalizer(deployContext *chetypes.DeployContext, finalizer string) error {
	if err := ReloadCheClusterCR(deployContext); err != nil {
		return err
	}

	if !utils.Contains(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
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

func DeleteFinalizer(deployContext *chetypes.DeployContext, finalizer string) error {
	if utils.Contains(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		for {
			deployContext.CheCluster.ObjectMeta.Finalizers = utils.Remove(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer)
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

func DeleteObjectWithFinalizer(deployContext *chetypes.DeployContext, key client.ObjectKey, objectMeta client.Object, finalizer string) error {
	_, err := Delete(deployContext, key, objectMeta)
	if err != nil {
		// failed to delete, shouldn't us prevent from removing finalizer
		logrus.Error(err)
	}

	return DeleteFinalizer(deployContext, finalizer)
}
