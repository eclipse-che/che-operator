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

package test

import (
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewDWOPod returns a running DevWorkspace Operator controller pod
// that exposes the given project clone image.
func NewDWOPod(namespace string, projectCloneImage string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-controller",
			Namespace: namespace,
			Labels: map[string]string{
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "devworkspace-controller",
					Env: []corev1.EnvVar{
						{Name: constants.DevWorkspaceProjectCloneImageEnvName, Value: projectCloneImage},
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}
