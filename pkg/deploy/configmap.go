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
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncConfigMapToCluster(checluster *orgv1.CheCluster, specConfigMap *corev1.ConfigMap, clusterAPI ClusterAPI) (*corev1.ConfigMap, error) {
	clusterConfigMap, err := getClusterConfigMap(specConfigMap.Name, specConfigMap.Namespace, clusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterConfigMap == nil {
		logrus.Infof("Creating a new object: %s, name %s", specConfigMap.Kind, specConfigMap.Name)
		err := clusterAPI.Client.Create(context.TODO(), specConfigMap)
		return nil, err
	}

	diff := cmp.Diff(clusterConfigMap.Data, specConfigMap.Data)
	if len(diff) > 0 {
		logrus.Infof("Updating existing object: %s, name: %s", specConfigMap.Kind, specConfigMap.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterConfigMap.Data = specConfigMap.Data
		err := clusterAPI.Client.Update(context.TODO(), clusterConfigMap)
		return nil, err
	}

	return clusterConfigMap, nil
}

func GetSpecConfigMap(
	checluster *orgv1.CheCluster,
	name string,
	data map[string]string,
	clusterAPI ClusterAPI) (*corev1.ConfigMap, error) {

	labels := GetLabels(checluster, DefaultCheFlavor(checluster))
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Data: data,
	}

	if !util.IsTestMode() {
		err := controllerutil.SetControllerReference(checluster, configMap, clusterAPI.Scheme)
		if err != nil {
			return nil, err
		}
	}

	return configMap, nil
}

func getClusterConfigMap(name string, namespace string, client runtimeClient.Client) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return configMap, nil
}
