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
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Container", func() {
	var container *corev1.Container

	BeforeEach(func() {
		container = &corev1.Container{Name: "test"}
	})

	Describe("ApplyOverrides", func() {
		It("should do nothing when overrides are nil", func() {
			container.Image = "test/test:next"
			Container.ApplyOverrides(container, nil)
			Expect(container.Image).To(Equal("test/test:next"))
		})

		Context("Image", func() {
			It("should override image", func() {
				Container.ApplyOverrides(container, &chev2.Container{Image: "custom/image:1.0.0"})
				Expect(container.Image).To(Equal("custom/image:1.0.0"))
			})

			It("should keep image when not overridden", func() {
				container.Image = "test/test:next"
				Container.ApplyOverrides(container, &chev2.Container{Env: []corev1.EnvVar{{Name: "test", Value: "test"}}})
				Expect(container.Image).To(Equal("test/test:next"))
			})
		})

		Context("ImagePullPolicy", func() {
			It("should use ImagePullPolicy set explicitly", func() {
				container.Image = "test/test:test"
				container.ImagePullPolicy = corev1.PullIfNotPresent

				Container.ApplyOverrides(container, &chev2.Container{ImagePullPolicy: corev1.PullNever})
				Expect(container.ImagePullPolicy).To(Equal(corev1.PullNever))
			})

			It("should set ImagePullPolicy from image if not set", func() {
				container.Image = "test/test:next"

				Container.ApplyOverrides(container, &chev2.Container{})
				Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))
			})

			It("should update ImagePullPolicy if image is overridden", func() {
				container.Image = "test/test:next"
				container.ImagePullPolicy = corev1.PullAlways

				Container.ApplyOverrides(container, &chev2.Container{Image: "test/test:1.0.0"})
				Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			})

			It("should update ImagePullPolicy to Always for next tag", func() {
				container.Image = "test/test:test"

				Container.ApplyOverrides(container, &chev2.Container{Image: "test/test:next"})
				Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))
			})

			It("should keep ImagePullPolicy if neither image nor ImagePullPolicy is overridden", func() {
				container.Image = "test/test:next"
				container.ImagePullPolicy = corev1.PullIfNotPresent

				Container.ApplyOverrides(container, &chev2.Container{Env: []corev1.EnvVar{{Name: "test", Value: "test"}}})
				Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			})
		})

		Context("Env", func() {
			It("should merge env variables by name", func() {
				container.Env = []corev1.EnvVar{
					{Name: "env_1", Value: "value_1"},
					{Name: "env_3", Value: "value_3"},
				}

				Container.ApplyOverrides(container, &chev2.Container{
					Env: []corev1.EnvVar{
						{Name: "env_1", Value: "value_2"},
						{Name: "env_2", Value: "value_2"},
					},
				})

				Expect(container.Env).To(Equal([]corev1.EnvVar{
					{Name: "env_1", Value: "value_2"},
					{Name: "env_3", Value: "value_3"},
					{Name: "env_2", Value: "value_2"},
				}))
			})
		})

		Context("VolumeMounts", func() {
			It("should merge volume mounts by name", func() {
				container.VolumeMounts = []corev1.VolumeMount{
					{Name: "vm-1", MountPath: "/a"},
					{Name: "vm-3", MountPath: "/c"},
				}

				Container.ApplyOverrides(container, &chev2.Container{
					VolumeMounts: []corev1.VolumeMount{
						{Name: "vm-1", MountPath: "/b"},
						{Name: "vm-2", MountPath: "/d"},
					},
				})

				Expect(container.VolumeMounts).To(Equal([]corev1.VolumeMount{
					{Name: "vm-1", MountPath: "/b"},
					{Name: "vm-3", MountPath: "/c"},
					{Name: "vm-2", MountPath: "/d"},
				}))
			})
		})

		Context("Resources", func() {
			It("should set requests and limits when not set", func() {
				Container.ApplyOverrides(container, &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Requests: &chev2.ResourceList{
							Memory: new(resource.MustParse("100Mi")),
						},
						Limits: &chev2.ResourceList{
							Memory: new(resource.MustParse("200Mi")),
						},
					},
				})

				Expect(container.Resources.Requests.Memory().String()).To(Equal("100Mi"))
				Expect(container.Resources.Limits.Memory().String()).To(Equal("200Mi"))
			})

			It("should override CPU and memory values", func() {
				container.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("100Mi"),
						corev1.ResourceCPU:    resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("200Mi"),
						corev1.ResourceCPU:    resource.MustParse("2"),
					},
				}

				Container.ApplyOverrides(container, &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Limits: &chev2.ResourceList{
							Cpu: new(resource.MustParse("500m")),
						},
					},
				})

				Expect(container.Resources.Requests.Cpu().String()).To(Equal("1"))
				Expect(container.Resources.Requests.Memory().String()).To(Equal("100Mi"))
				Expect(container.Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(container.Resources.Limits.Memory().String()).To(Equal("200Mi"))
			})

			It("should remove entry when override value is zero", func() {
				container.Resources = corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("200Mi"),
						corev1.ResourceCPU:    resource.MustParse("2"),
					},
				}

				Container.ApplyOverrides(container, &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Limits: &chev2.ResourceList{
							Memory: new(resource.MustParse("0")),
						},
					},
				})

				Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceCPU))
				Expect(container.Resources.Limits).ToNot(HaveKey(corev1.ResourceMemory))
			})

			It("should remove CPU entry when override value is zero", func() {
				container.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				}

				Container.ApplyOverrides(container, &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Requests: &chev2.ResourceList{
							Cpu: new(resource.MustParse("0")),
						},
					},
				})

				Expect(container.Resources.Requests).To(BeNil())
			})
		})

		Context("SecurityContext", func() {
			It("should merge security context fields", func() {
				runAs := int64(1001)
				container.SecurityContext = &corev1.SecurityContext{
					AllowPrivilegeEscalation: new(false),
				}

				Container.ApplyOverrides(container, &chev2.Container{
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: &runAs,
					},
				})

				Expect(container.SecurityContext.RunAsUser).To(Equal(new(runAs)))
				Expect(container.SecurityContext.AllowPrivilegeEscalation).To(Equal(new(false)))
			})

			It("should create security context when not set", func() {
				Expect(container.SecurityContext).To(BeNil())

				Container.ApplyOverrides(container, &chev2.Container{
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: new(true),
					},
				})

				Expect(container.SecurityContext).ToNot(BeNil())
				Expect(container.SecurityContext.RunAsNonRoot).To(Equal(new(true)))
			})

			It("should override all security context fields", func() {
				overrides := &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_ADMIN"},
					},
					Privileged:               new(true),
					SELinuxOptions:           &corev1.SELinuxOptions{User: "system_u"},
					WindowsOptions:           &corev1.WindowsSecurityContextOptions{GMSACredentialSpecName: new("gmsa")},
					RunAsUser:                new(int64(1001)),
					RunAsGroup:               new(int64(1001)),
					RunAsNonRoot:             new(true),
					ReadOnlyRootFilesystem:   new(true),
					AllowPrivilegeEscalation: new(false),
					ProcMount:                new(corev1.DefaultProcMount),
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					AppArmorProfile:          &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeRuntimeDefault},
				}

				Container.ApplyOverrides(container, &chev2.Container{SecurityContext: overrides})

				Expect(container.SecurityContext).To(Equal(overrides))
			})
		})
	})

	Describe("TrimCPULimitIfNoLimitRange", func() {
		It("should remove CPU limit when no LimitRange exists", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().Build()
			container.Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			}

			Expect(Container.TrimCPULimitIfNoLimitRange(cheCtx, cheCtx.CheCluster.Namespace, container)).To(Succeed())
			Expect(container.Resources.Limits).ToNot(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceMemory))
		})

		It("should keep CPU limit when LimitRange exists", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtxWithLimitRange := test.NewCheContextBuilder().WithObjects(
				&corev1.LimitRange{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "eclipse-che",
					},
				},
			).Build()

			container.Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("250m"),
				},
			}

			Expect(Container.TrimCPULimitIfNoLimitRange(cheCtxWithLimitRange, cheCtxWithLimitRange.CheCluster.Namespace, container)).To(Succeed())
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("250m"))
		})
	})
})
