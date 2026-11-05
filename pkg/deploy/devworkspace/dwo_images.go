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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const projectCloneImageEnvName = "RELATED_IMAGE_project_clone"

// GetProjectCloneImage returns the image of the project clone init container that the
// DevWorkspace Operator adds to workspace pods. Unless overridden in the CheCluster, the image
// is read from the running DevWorkspace Operator. Returns an empty string if it is not found.
func GetProjectCloneImage(cheCtx *chetypes.CheContext) (string, error) {
	projectCloneContainer := cheCtx.CheCluster.Spec.DevEnvironments.ProjectCloneContainer
	if projectCloneContainer != nil && projectCloneContainer.Image != "" {
		return projectCloneContainer.Image, nil
	}

	if cheCtx.DWONamespace == "" {
		return "", nil
	}

	selector := labels.SelectorFromSet(
		labels.Set{
			constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
			constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
		},
	)

	items, err := cheCtx.ClusterAPI.ClientWrapper.List(
		cheCtx.Context,
		&corev1.PodList{},
		&client.ListOptions{LabelSelector: selector, Namespace: cheCtx.DWONamespace},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list DevWorkspace operator pods in %s: %w", cheCtx.DWONamespace, err)
	}

	for _, item := range items {
		pod, ok := item.(*corev1.Pod)
		if !ok {
			continue
		}

		for _, container := range pod.Spec.Containers {
			for _, env := range container.Env {
				if env.Name == projectCloneImageEnvName && env.Value != "" {
					return env.Value, nil
				}
			}
		}
	}

	return "", nil
}
