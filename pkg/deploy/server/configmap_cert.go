//
// Copyright (c) 2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package server

import (
	"context"

	"github.com/eclipse/che-operator/pkg/deploy"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	injector = "config.openshift.io/inject-trusted-cabundle"
)

func SyncTrustStoreConfigMapToCluster(deployContext *deploy.DeployContext) (*corev1.ConfigMap, error) {
	name := deployContext.CheCluster.Spec.Server.ServerTrustStoreConfigMapName
	specConfigMap, err := deploy.GetSpecConfigMap(deployContext, name, map[string]string{}, deploy.DefaultCheFlavor(deployContext.CheCluster))
	if err != nil {
		return nil, err
	}

	// OpenShift will automatically injects all certs into the configmap
	specConfigMap.ObjectMeta.Labels[injector] = "true"

	clusterConfigMap, err := deploy.GetClusterConfigMap(specConfigMap.Name, specConfigMap.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterConfigMap == nil {
		logrus.Infof("Creating a new object: %s, name %s", specConfigMap.Kind, specConfigMap.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specConfigMap)
		return nil, err
	}

	if clusterConfigMap.ObjectMeta.Labels[injector] != "true" {
		clusterConfigMap.ObjectMeta.Labels[injector] = "true"
		logrus.Infof("Updating existed object: %s, name: %s", specConfigMap.Kind, specConfigMap.Name)
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterConfigMap)
		return nil, err
	}

	return clusterConfigMap, nil
}
