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
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncServiceAccountToCluster(checluster *orgv1.CheCluster, name string, clusterAPI ClusterAPI) (*corev1.ServiceAccount, error) {
	specSA, err := getSpecServiceAccount(checluster, name, clusterAPI)
	if err != nil {
		return nil, err
	}

	clusterSA, err := getClusterServiceAccount(specSA.Name, specSA.Namespace, clusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterSA == nil {
		logrus.Infof("Creating a new object: %s, name %s", specSA.Kind, specSA.Name)
		err := clusterAPI.Client.Create(context.TODO(), specSA)
		return nil, err
	}

	return clusterSA, nil
}

func getClusterServiceAccount(name string, namespace string, client runtimeClient.Client) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, serviceAccount)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return serviceAccount, nil
}

func getSpecServiceAccount(checluster *orgv1.CheCluster, name string, clusterAPI ClusterAPI) (*corev1.ServiceAccount, error) {
	labels := GetLabels(checluster, util.GetValue(checluster.Spec.Server.CheFlavor, DefaultCheFlavor))
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
	}

	err := controllerutil.SetControllerReference(checluster, serviceAccount, clusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return serviceAccount, nil
}
