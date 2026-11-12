//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDevWorkspaceOperatorNamespace(context context.Context, clientWrapper clientwrapper.ClientWrapper) (string, error) {
	items, err := clientWrapper.List(
		context,
		&corev1.PodList{},
		&client.ListOptions{LabelSelector: devWorkspaceOperatorPodSelector},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list DevWorkspace operator pods: %w", err)
	}

	devWorkspaceOperatorNamespace := ""

	for _, item := range items {
		pod, ok := item.(*corev1.Pod)
		if !ok || !IsDevWorkspaceOperatorPod(pod) {
			continue
		}

		if devWorkspaceOperatorNamespace != "" && devWorkspaceOperatorNamespace != pod.Namespace {
			return "", fmt.Errorf("multiple DevWorkspace Operator pods were found across different namespaces")
		}
		devWorkspaceOperatorNamespace = pod.Namespace
	}

	return devWorkspaceOperatorNamespace, nil
}
