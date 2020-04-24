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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/sirupsen/logrus"
)

func UpdateCheCRSpec(instance *orgv1.CheCluster, updatedField string, value string, clusterAPI ClusterAPI) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", instance.Name, updatedField, value)
	err = clusterAPI.Client.Update(context.TODO(), instance)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", instance.Name)
	return nil
}
