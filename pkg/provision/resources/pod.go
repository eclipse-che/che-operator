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

func (h *podHandler) EnsureSecurityStandards(pod *corev1.PodSpec, userId int64, groupId int64) {
	h.ensureSecurityStandardsForContainers(pod.Containers)
	h.ensureSecurityStandardsForContainers(pod.InitContainers)

	if pod.SecurityContext == nil {
		pod.SecurityContext = &corev1.PodSecurityContext{}
	}

	if !infrastructure.IsOpenShift() {
		pod.SecurityContext.RunAsUser = new(userId)
		pod.SecurityContext.FSGroup = new(groupId)
	} else {
		if pod.SecurityContext.SeccompProfile == nil {
			pod.SecurityContext.SeccompProfile = &corev1.SeccompProfile{}
		}
		pod.SecurityContext.SeccompProfile.Type = corev1.SeccompProfileTypeRuntimeDefault
	}
}

func (h *podHandler) ensureSecurityStandardsForContainers(containers []corev1.Container) {
	for idx := range containers {
		if containers[idx].SecurityContext == nil {
			containers[idx].SecurityContext = &corev1.SecurityContext{}
		}
		containers[idx].SecurityContext.AllowPrivilegeEscalation = new(false)
		containers[idx].SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
		containers[idx].SecurityContext.RunAsNonRoot = new(true)
	}
}
