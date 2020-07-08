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
package deploy

import (
	"context"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	injector = "config.openshift.io/inject-trusted-cabundle"
)

func SyncTrustStoreConfigMapToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) (*corev1.ConfigMap, error) {
	name := checluster.Spec.Server.ServerTrustStoreConfigMapName
	specConfigMap, err := GetSpecConfigMap(checluster, name, map[string]string{}, clusterAPI)
	if err != nil {
		return nil, err
	}

	// OpenShift will automatically injects all certs into the configmap
	specConfigMap.ObjectMeta.Labels[injector] = "true"

	clusterConfigMap, err := getClusterConfigMap(specConfigMap.Name, specConfigMap.Namespace, clusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterConfigMap == nil {
		logrus.Infof("Creating a new object: %s, name %s", specConfigMap.Kind, specConfigMap.Name)
		err := clusterAPI.Client.Create(context.TODO(), specConfigMap)
		return nil, err
	}

	if clusterConfigMap.ObjectMeta.Labels[injector] != "true" {
		clusterConfigMap.ObjectMeta.Labels[injector] = "true"
		logrus.Infof("Updating existed object: %s, name: %s", specConfigMap.Kind, specConfigMap.Name)
		err := clusterAPI.Client.Update(context.TODO(), clusterConfigMap)
		return nil, err
	}

	return clusterConfigMap, nil
}
