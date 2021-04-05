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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SyncServiceAccountToCluster(deployContext *DeployContext, name string) (bool, error) {
	saSpec := getServiceAccountSpec(deployContext, name)
	return CreateIfNotExists(deployContext, saSpec)
}

func getServiceAccountSpec(deployContext *DeployContext, name string) *corev1.ServiceAccount {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
	}

	return serviceAccount
}
