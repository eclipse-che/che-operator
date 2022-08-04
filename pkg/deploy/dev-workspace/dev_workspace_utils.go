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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func isCheOperatorHasOwner(ctx *chetypes.DeployContext) (bool, error) {
	operatorNamespace, err := utils.GetOperatorNamespace()
	if err != nil {
		return false, err
	}

	deployment := &appsv1.Deployment{}
	if err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: operatorNamespace,
			Name:      defaults.GetCheFlavor() + "-operator",
		},
		deployment); err != nil {
		return false, err
	}

	return len(deployment.OwnerReferences) != 0, nil
}

func isDevWorkspaceOperatorHasOwner(ctx *chetypes.DeployContext) (bool, error) {
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

	for _, deployment := range deployments.Items {
		if len(deployment.OwnerReferences) != 0 {
			return true, nil
		}
	}

	return false, nil
}
