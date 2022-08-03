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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Indicates if Web Terminal installed on the cluster by checking
// its subscription in both `openshift-operators` and current namespaces
func isWebTerminalInstalledByOlm(deployContext *chetypes.DeployContext) (bool, error) {
	cheOperatorNamespace, err := utils.GetOperatorNamespace()
	if err != nil {
		return false, err
	}

	namespace2check := []string{cheOperatorNamespace, OperatorNamespace}
	for _, namespace := range namespace2check {
		isWebTerminalSubscriptionExist, err := isSubscriptionExist(WebTerminalOperatorSubscriptionName, namespace, deployContext)
		if isWebTerminalSubscriptionExist {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}

	return false, nil
}

func isSubscriptionExist(name string, namespace string, ctx *chetypes.DeployContext) (bool, error) {
	if !utils.IsK8SResourceServed(ctx.ClusterAPI.DiscoveryClient, SubscriptionResourceName) {
		return false, nil
	}

	if err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
		&operatorsv1alpha1.Subscription{}); err != nil {

		if errors.IsNotFound(err) {
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

// Indicates if Eclipse Che operator installed by OLM by checking existence `olm.owner` label.
func isCheOperatorInstalledByOLM(ctx *chetypes.DeployContext) (bool, error) {
	operatorNamespace, err := utils.GetOperatorNamespace()
	if err != nil {
		return false, err
	}

	deployment := &appsv1.Deployment{}
	if err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{Namespace: operatorNamespace, Name: defaults.GetCheFlavor() + "-operator"},
		deployment); err != nil {
		return false, err
	}

	return deployment.Labels[constants.OlmOwnerLabelKey] != "", nil
}

func isDevWorkspaceOperatorInstalledByOLM(ctx *chetypes.DeployContext) (bool, error) {
	deployments := &appsv1.DeploymentList{}
	if err := ctx.ClusterAPI.NonCachingClient.List(
		context.TODO(),
		deployments,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperator,
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceController,
			}),
		}); err != nil {
		return false, err
	}

	if len(deployments.Items) != 1 {
		return false, nil
	}

	return deployments.Items[0].Labels[constants.OlmOwnerLabelKey] != "", nil
}
