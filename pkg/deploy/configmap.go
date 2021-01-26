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

// SyncConfigMapToCluster makes sure that given config map spec is actual.
// It compares config map data and labels.
// If returned config map is nil then it means that the config map update is in progress and reconcile loop probably should be restarted.
func SyncConfigMapToCluster(deployContext *DeployContext, specConfigMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	clusterConfigMap, err := GetClusterConfigMap(specConfigMap.Name, specConfigMap.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterConfigMap == nil {
		logrus.Infof("Creating a new object: %s, name %s", specConfigMap.Kind, specConfigMap.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specConfigMap)
		return nil, err
	}

	dataDiff := cmp.Diff(clusterConfigMap.Data, specConfigMap.Data)
	labelsDiff := cmp.Diff(clusterConfigMap.ObjectMeta.Labels, specConfigMap.ObjectMeta.Labels)
	if len(dataDiff) > 0 || len(labelsDiff) > 0 {
		logrus.Infof("Updating existing object: %s, name: %s", specConfigMap.Kind, specConfigMap.Name)
		fmt.Printf("Difference:\n%s\n%s", dataDiff, labelsDiff)
		clusterConfigMap.Data = specConfigMap.Data
		clusterConfigMap.ObjectMeta.Labels = specConfigMap.ObjectMeta.Labels
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterConfigMap)
		return nil, err
	}

	return clusterConfigMap, nil
}

// GetSpecConfigMap returns config map spec template
func GetSpecConfigMap(
	deployContext *DeployContext,
	name string,
	data map[string]string,
	component string) (*corev1.ConfigMap, error) {

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    GetLabels(deployContext.CheCluster, component),
		},
		Data: data,
	}

	if !util.IsTestMode() {
		err := controllerutil.SetControllerReference(deployContext.CheCluster, configMap, deployContext.ClusterAPI.Scheme)
		if err != nil {
			return nil, err
		}
	}

	return configMap, nil
}

// GetClusterConfigMap reads config map from cluster
func GetClusterConfigMap(name string, namespace string, client runtimeClient.Client) (*corev1.ConfigMap, error) {
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
