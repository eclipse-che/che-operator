//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
)

func CleanUpAllFinalizers(cheCtx *chetypes.CheContext) error {
	cheCtx.CheCluster.Finalizers = []string{}
	return cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
}

func AppendFinalizer(cheCtx *chetypes.CheContext, finalizer string) error {
	if err := ReloadCheClusterCR(cheCtx); err != nil {
		return err
	}

	if !utils.Contains(cheCtx.CheCluster.Finalizers, finalizer) {
		for {
			cheCtx.CheCluster.Finalizers = append(cheCtx.CheCluster.Finalizers, finalizer)
			err := cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
			if err == nil {
				logrus.Infof("Added finalizer: %s", finalizer)
				return nil
			} else if !errors.IsConflict(err) {
				return err
			}

			err = ReloadCheClusterCR(cheCtx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteFinalizer(cheCtx *chetypes.CheContext, finalizer string) error {
	if utils.Contains(cheCtx.CheCluster.Finalizers, finalizer) {
		for {
			cheCtx.CheCluster.Finalizers = utils.Remove(cheCtx.CheCluster.Finalizers, finalizer)
			err := cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
			if err == nil {
				logrus.Infof("Deleted finalizer: %s", finalizer)
				return nil
			} else if !errors.IsConflict(err) {
				return err
			}

			err = ReloadCheClusterCR(cheCtx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
