//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDevWorkspaceOperatorNamespace(clientWrapper *k8sclient.K8sClientWrapper) (string, error) {
	selector := labels.SelectorFromSet(
		labels.Set{
			constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
			constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
		},
	)

	items, err := clientWrapper.List(
		context.TODO(),
		&corev1.PodList{},
		&client.ListOptions{LabelSelector: selector},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list DevWorkspace operator pods: %w", err)
	}

	devWorkspaceOperatorNamespace := ""

	for _, item := range items {
		pod := item.(*corev1.Pod)
		if pod.Spec.ServiceAccountName == constants.DevWorkspaceServiceAccountName {
			if devWorkspaceOperatorNamespace != "" && devWorkspaceOperatorNamespace != pod.Namespace {
				return "", fmt.Errorf("multiple DevWorkspace Operator pods were found across different namespaces")
			}
			devWorkspaceOperatorNamespace = pod.Namespace
		}
	}

	if devWorkspaceOperatorNamespace != "" {
		return devWorkspaceOperatorNamespace, nil
	}

	return "", fmt.Errorf("DevWorkspace operator namespace not found")
}
