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
)

func AppendFinalizer(deployContext *DeployContext, finalizer string) error {
	if !util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		logrus.Infof("Adding %s finalizer", finalizer)
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
	logrus.Infof("Removing %s finalizer", finalizer)
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
