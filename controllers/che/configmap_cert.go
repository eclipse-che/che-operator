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

package che

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	injector = "config.openshift.io/inject-trusted-cabundle"
)

func SyncTrustStoreConfigMapToCluster(deployContext *deploy.DeployContext) (bool, error) {
	name := deployContext.CheCluster.Spec.Server.ServerTrustStoreConfigMapName
	configMapSpec := deploy.GetConfigMapSpec(deployContext, name, map[string]string{}, deploy.DefaultCheFlavor(deployContext.CheCluster))

	// OpenShift will automatically injects all certs into the configmap
	configMapSpec.ObjectMeta.Labels[injector] = "true"

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, name, actual)
	if err != nil {
		return false, err
	}

	if !exists {
		// We have to create an empty config map with the specific labels
		return deploy.Create(deployContext, configMapSpec)
	}

	if actual.ObjectMeta.Labels[injector] != "true" {
		actual.ObjectMeta.Labels[injector] = "true"
		logrus.Infof("Updating existed object: %s, name: %s", configMapSpec.Kind, configMapSpec.Name)
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), actual)
		return true, err
	}

	return true, nil
}
