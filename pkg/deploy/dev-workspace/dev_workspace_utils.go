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

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isDevWorkspaceOperatorCSVExists(deployContext *chetypes.DeployContext) bool {
	// If clusterserviceversions resource doesn't exist in cluster DWO as well will not be present
	if !utils.IsK8SResourceServed(deployContext.ClusterAPI.DiscoveryClient, ClusterServiceVersionResourceName) {
		return false
	}

	csvList := &operatorsv1alpha1.ClusterServiceVersionList{}
	err := deployContext.ClusterAPI.NonCachingClient.List(context.TODO(), csvList, &client.ListOptions{Namespace: OperatorNamespace})
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

func isWebTerminalSubscriptionExist(deployContext *chetypes.DeployContext) (bool, error) {
	// If subscriptions resource doesn't exist in cluster WTO as well will not be present
	if !utils.IsK8SResourceServed(deployContext.ClusterAPI.DiscoveryClient, SubscriptionResourceName) {
		return false, nil
	}

	subscription := &operatorsv1alpha1.Subscription{}
	if err := deployContext.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: OperatorNamespace,
		},
		subscription); err != nil {

		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func createDwNamespace(deployContext *chetypes.DeployContext) (bool, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Spec: corev1.NamespaceSpec{},
	}

	return deploy.CreateIfNotExists(deployContext, namespace)
}

func isOnlyOneOperatorManagesDWResources(deployContext *chetypes.DeployContext) (bool, error) {
	cheClusters := &chev2.CheClusterList{}
	err := deployContext.ClusterAPI.NonCachingClient.List(context.TODO(), cheClusters)
	if err != nil {
		return false, err
	}

	return len(cheClusters.Items) == 1, nil
}
