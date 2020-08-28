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

	"github.com/sirupsen/logrus"
)

func UpdateCheCRSpec(deployContext *DeployContext, updatedField string, value string) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", deployContext.CheCluster.Name, updatedField, value)
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", deployContext.CheCluster.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", deployContext.CheCluster.Name)
	return nil
}

func UpdateCheCRStatus(deployContext *DeployContext, updatedField string, value string) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", deployContext.CheCluster.Name, updatedField, value)
	err = deployContext.ClusterAPI.Client.Status().Update(context.TODO(), deployContext.CheCluster)
	if err != nil {
		logrus.Errorf("Failed to update %s CR. Fetching the latest CR version: %s", deployContext.CheCluster.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", deployContext.CheCluster.Name)
	return nil
}
