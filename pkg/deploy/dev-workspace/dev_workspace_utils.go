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

package devworkspace

import (
	"context"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isDevWorkspaceDeploymentExists(deployContext *deploy.DeployContext) (bool, error) {
	return deploy.Get(deployContext, types.NamespacedName{
		Namespace: DevWorkspaceNamespace,
		Name:      DevWorkspaceDeploymentName,
	}, &appsv1.Deployment{})
}

func isDevWorkspaceOperatorCSVExists(deployContext *deploy.DeployContext) bool {
	// If clusterserviceversions resource doesn't exist in cluster DWO as well will not be present
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, ClusterServiceVersionResourceName) {
		return false
	}

	csvList := &operatorsv1alpha1.ClusterServiceVersionList{}
	err := deployContext.ClusterAPI.Client.List(context.TODO(), csvList, &client.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list csv: %v", err)
		return false
	}

	for _, csv := range csvList.Items {
		if strings.HasPrefix(csv.Name, DevWorkspaceCSVNamePrefix) {
			return true
		}
	}

	return false
}

func isWebTerminalSubscriptionExist(deployContext *deploy.DeployContext) (bool, error) {
	// If subscriptions resource doesn't exist in cluster WTO as well will not be present
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, SubscriptionResourceName) {
		return false, nil
	}

	subscription := &operatorsv1alpha1.Subscription{}
	if err := deployContext.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		subscription); err != nil {

		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func createDwNamespace(deployContext *deploy.DeployContext) (bool, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Spec: corev1.NamespaceSpec{},
	}

	return deploy.CreateIfNotExists(deployContext, namespace)
}

func isOnlyOneOperatorManagesDWResources(deployContext *deploy.DeployContext) (bool, error) {
	cheClusters := &orgv1.CheClusterList{}
	err := deployContext.ClusterAPI.NonCachingClient.List(context.TODO(), cheClusters)
	if err != nil {
		return false, err
	}

	return len(cheClusters.Items) == 1, nil
}
