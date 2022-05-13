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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
)

// UpdateCheCRSpec - updates Che CR "spec" by field
func UpdateCheCRSpec(deployContext *chetypes.DeployContext, field string, value string) error {
	err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
	if err == nil {
		logrus.Infof("Custom resource spec %s updated with %s: %s", deployContext.CheCluster.Name, field, value)
		return nil
	}
	return err
}

func UpdateCheCRStatus(deployContext *chetypes.DeployContext, field string, value string) (err error) {
	err = deployContext.ClusterAPI.Client.Status().Update(context.TODO(), deployContext.CheCluster)
	if err == nil {
		logrus.Infof("Custom resource status %s updated with %s: %s", deployContext.CheCluster.Name, field, value)
		return nil
	}

	return err
}

func SetStatusDetails(deployContext *chetypes.DeployContext, reason string, message string) (err error) {
	if reason != deployContext.CheCluster.Status.Reason {
		deployContext.CheCluster.Status.Reason = reason
		if err := UpdateCheCRStatus(deployContext, "status: Reason", reason); err != nil {
			return err
		}
	}
	if message != deployContext.CheCluster.Status.Message {
		deployContext.CheCluster.Status.Message = message
		if err := UpdateCheCRStatus(deployContext, "status: Message", message); err != nil {
			return err
		}
	}
	return nil
}

func ReloadCheClusterCR(deployContext *chetypes.DeployContext) error {
	return deployContext.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: deployContext.CheCluster.Name, Namespace: deployContext.CheCluster.Namespace},
		deployContext.CheCluster)
}
