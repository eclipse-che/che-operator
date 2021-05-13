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
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// UpdateCheCRSpec - updates Che CR "spec" by field
func UpdateCheCRSpec(deployContext *DeployContext, field string, value string) error {
	for {
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
		if err == nil {
			logrus.Infof("Custom resource %s updated with %s: %s", deployContext.CheCluster.Name, field, value)
			return nil
		} else if !errors.IsConflict(err) {
			return err
		}

		err = ReloadCheCluster(deployContext)
		if err != nil {
			return err
		}
	}
}

// UpdateCheCRSpecByFields - updates Che CR "spec" fields by field map
func UpdateCheCRSpecByFields(deployContext *DeployContext, fields map[string]string) (err error) {
	updateInfo := []string{}
	for updatedField, value := range fields {
		updateInfo = append(updateInfo, fmt.Sprintf("%s: %s", updatedField, value))
	}

	for {
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), deployContext.CheCluster)
		if err == nil {
			logrus.Infof(fmt.Sprintf("Custom resource %s updated with: ", deployContext.CheCluster.Name) + strings.Join(updateInfo, ", "))
			return nil
		} else if !errors.IsConflict(err) {
			return err
		}

		err = ReloadCheCluster(deployContext)
		if err != nil {
			return err
		}
	}
}

func UpdateCheCRStatus(deployContext *DeployContext, field string, value string) (err error) {
	for {
		err = deployContext.ClusterAPI.Client.Status().Update(context.TODO(), deployContext.CheCluster)
		if err == nil {
			logrus.Infof("Custom resource %s updated with %s: %s", deployContext.CheCluster.Name, field, value)
			return nil
		} else if !errors.IsConflict(err) {
			return err
		}

		err = ReloadCheCluster(deployContext)
		if err != nil {
			return err
		}
	}
}

func SetStatusDetails(deployContext *DeployContext, reason string, message string, helpLink string) (err error) {
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
	if helpLink != deployContext.CheCluster.Status.HelpLink {
		deployContext.CheCluster.Status.HelpLink = helpLink
		if err := UpdateCheCRStatus(deployContext, "status: HelpLink", message); err != nil {
			return err
		}
	}
	return nil
}

func ReloadCheCluster(deployContext *DeployContext) error {
	return deployContext.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: deployContext.CheCluster.Name, Namespace: deployContext.CheCluster.Namespace},
		deployContext.CheCluster)
}
