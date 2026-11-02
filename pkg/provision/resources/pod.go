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

package resources

import (
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	corev1 "k8s.io/api/core/v1"
)

var Pod = &podHandler{}

type podHandler struct{}

func (h podHandler) EnsureSecurityStandards(
	podSpec *corev1.PodSpec,
	defaultUserId int64,
	defaultGroupId int64,
) {
	for i := range podSpec.Containers {
		if podSpec.Containers[i].SecurityContext == nil {
			podSpec.Containers[i].SecurityContext = &corev1.SecurityContext{}
		}

		podSpec.Containers[i].SecurityContext.AllowPrivilegeEscalation = new(false)
		podSpec.Containers[i].SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
		podSpec.Containers[i].SecurityContext.RunAsNonRoot = new(true)
	}

	for i := range podSpec.InitContainers {
		if podSpec.InitContainers[i].SecurityContext == nil {
			podSpec.InitContainers[i].SecurityContext = &corev1.SecurityContext{}
		}

		podSpec.InitContainers[i].SecurityContext.AllowPrivilegeEscalation = new(false)
		podSpec.InitContainers[i].SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
		podSpec.InitContainers[i].SecurityContext.RunAsNonRoot = new(true)
	}

	if podSpec.SecurityContext == nil {
		podSpec.SecurityContext = &corev1.PodSecurityContext{}
	}

	if !infrastructure.IsOpenShift() {
		podSpec.SecurityContext.RunAsUser = new(defaultUserId)
		podSpec.SecurityContext.FSGroup = new(defaultGroupId)
	} else {
		if podSpec.SecurityContext.SeccompProfile == nil {
			podSpec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
		}
		podSpec.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
	}
}
