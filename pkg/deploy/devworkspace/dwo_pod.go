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
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// devWorkspaceOperatorPodSelector selects the DevWorkspace Operator controller pods by their labels.
var devWorkspaceOperatorPodSelector = labels.SelectorFromSet(
	labels.Set{
		constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
		constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
	},
)

// IsDevWorkspaceOperatorPod returns true if the object is a DevWorkspace Operator controller pod.
func IsDevWorkspaceOperatorPod(obj client.Object) bool {
	pod, ok := obj.(*corev1.Pod)

	return ok &&
		devWorkspaceOperatorPodSelector.Matches(labels.Set(pod.Labels)) &&
		pod.Spec.ServiceAccountName == constants.DevWorkspaceServiceAccountName
}
