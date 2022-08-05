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
	"os"
	"strconv"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func isNoOptDWO() (bool, error) {
	value, exists := os.LookupEnv("NO_OPT_DWO")
	if !exists {
		return false, nil
	}

	return strconv.ParseBool(value)
}

func isDevWorkspaceOperatorHasOwner(ctx *deploy.DeployContext) (bool, error) {
	deployments := &appsv1.DeploymentList{}
	if err := ctx.ClusterAPI.NonCachingClient.List(
		context.TODO(),
		deployments,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				deploy.KubernetesPartOfLabelKey: deploy.DevWorkspaceOperator,
				deploy.KubernetesNameLabelKey:   deploy.DevWorkspaceController,
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
