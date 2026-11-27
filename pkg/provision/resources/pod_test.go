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
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Pod", func() {
	var pod *corev1.PodSpec

	BeforeEach(func() {
		pod = &corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "server"},
			},
			InitContainers: []corev1.Container{
				{Name: "init"},
			},
		}
	})

	assertContainersSecurityStandards := func(containers []corev1.Container) {
		for _, container := range containers {
			Expect(container.SecurityContext).ToNot(BeNil())
			Expect(container.SecurityContext.AllowPrivilegeEscalation).To(Equal(ptr.To(false)))
			Expect(container.SecurityContext.RunAsNonRoot).To(Equal(ptr.To(true)))
			Expect(container.SecurityContext.Capabilities).To(Equal(&corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}))
		}
	}

	Context("on Kubernetes", func() {
		BeforeEach(func() {
			test.StartOn(infrastructure.Kubernetes)
		})

		It("should set container security standards and run as the given user", func() {
			Pod.EnsureSecurityStandards(pod, 1001, 1002)

			assertContainersSecurityStandards(pod.Containers)
			assertContainersSecurityStandards(pod.InitContainers)

			Expect(pod.SecurityContext).ToNot(BeNil())
			Expect(pod.SecurityContext.RunAsUser).To(Equal(ptr.To(int64(1001))))
			Expect(pod.SecurityContext.FSGroup).To(Equal(ptr.To(int64(1002))))
		})
	})

	Context("on OpenShift", func() {
		BeforeEach(func() {
			test.StartOn(infrastructure.OpenShiftV4)
		})

		It("should set container security standards and runtime default seccomp profile", func() {
			Pod.EnsureSecurityStandards(pod, 1001, 1002)

			assertContainersSecurityStandards(pod.Containers)
			assertContainersSecurityStandards(pod.InitContainers)

			Expect(pod.SecurityContext).ToNot(BeNil())
			Expect(pod.SecurityContext.RunAsUser).To(BeNil())
			Expect(pod.SecurityContext.FSGroup).To(BeNil())
			Expect(pod.SecurityContext.SeccompProfile).ToNot(BeNil())
			Expect(pod.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
		})
	})
})
