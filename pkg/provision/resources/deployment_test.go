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
	"github.com/eclipse-che/che-operator/pkg/common/cheerrors"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newDeployment(name string, containers ...corev1.Container) *appsv1.Deployment {
	if len(containers) == 0 {
		containers = []corev1.Container{{}}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

var _ = Describe("Deployment", func() {
	Describe("ApplyOverrides", func() {
		var deployment *appsv1.Deployment

		BeforeEach(func() {
			deployment = newDeployment("test", corev1.Container{Name: "test"})
		})

		It("should do nothing when overrides are nil", func() {
			deployment.Spec.Template.Spec.Containers[0].Image = "test/test:next"

			Deployment.ApplyOverrides(deployment, nil)

			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("test/test:next"))
		})

		It("should not fail on empty overrides", func() {
			Deployment.ApplyOverrides(deployment, &chev2.Deployment{})
		})

		It("should apply container overrides to a single container", func() {
			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Image: "custom/image:1.0.0",
					},
				},
			})

			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("custom/image:1.0.0"))
		})

		It("should apply container overrides by name when there are several containers", func() {
			deployment = newDeployment("test",
				corev1.Container{Name: "server"},
				corev1.Container{Name: "gateway"},
			)

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name:  "gateway",
						Image: "custom/gateway:1.0.0",
					},
				},
			})

			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(BeEmpty())
			Expect(deployment.Spec.Template.Spec.Containers[1].Image).To(Equal("custom/gateway:1.0.0"))
		})

		It("should not update resources when container overrides have no resources", func() {
			deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Mi"),
					corev1.ResourceCPU:    resource.MustParse("1"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("200Mi"),
					corev1.ResourceCPU:    resource.MustParse("2"),
				},
			}

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Containers: []chev2.Container{{Name: "test"}},
			})

			Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("1"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("100Mi"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("2"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("200Mi"))
		})

		It("should apply env variable overrides", func() {
			deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{Name: "env_1", Value: "value_1"},
				{Name: "env_3", Value: "value_3"},
			}

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name: "test",
						Env: []corev1.EnvVar{
							{Name: "env_1", Value: "value_2"},
							{Name: "env_2", Value: "value_2"},
						},
					},
				},
			})

			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(Equal([]corev1.EnvVar{
				{Name: "env_1", Value: "value_2"},
				{Name: "env_3", Value: "value_3"},
				{Name: "env_2", Value: "value_2"},
			}))
		})

		It("should apply ImagePullPolicy set explicitly", func() {
			deployment.Spec.Template.Spec.Containers[0].Image = "test/test:test"
			deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name:            "test",
						ImagePullPolicy: corev1.PullNever,
					},
				},
			})

			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullNever))
		})

		It("should update ImagePullPolicy to Always for next tag", func() {
			deployment.Spec.Template.Spec.Containers[0].Image = "test/test:test"

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Image: "test/test:next",
					},
				},
			})

			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("should apply node selector", func() {
			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				NodeSelector: map[string]string{
					"a": "b",
					"c": "d",
				},
			})

			Expect(deployment.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{
				"a": "b",
				"c": "d",
			}))
		})

		It("should apply tolerations", func() {
			tolerations := []corev1.Toleration{
				{
					Key:      "a",
					Operator: corev1.TolerationOpEqual,
					Value:    "b",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "b",
					Operator: corev1.TolerationOpExists,
					Value:    "c",
					Effect:   corev1.TaintEffectNoExecute,
				},
			}

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{Tolerations: tolerations})
			Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
		})

		It("should append unknown volumes and keep existing ones", func() {
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
				{Name: "drop-me", VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			}

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Volumes: []corev1.Volume{
					{Name: "custom", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cm"},
						},
					}},
				},
			})

			Expect(deployment.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(deployment.Spec.Template.Spec.Volumes[0].Name).To(Equal("drop-me"))
			Expect(deployment.Spec.Template.Spec.Volumes[0].EmptyDir).ToNot(BeNil())
			Expect(deployment.Spec.Template.Spec.Volumes[1].Name).To(Equal("custom"))
			Expect(deployment.Spec.Template.Spec.Volumes[1].ConfigMap).ToNot(BeNil())
		})

		It("should replace volume definition when name matches", func() {
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
				{Name: "shared", VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			}

			Deployment.ApplyOverrides(deployment, &chev2.Deployment{
				Volumes: []corev1.Volume{
					{Name: "shared", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cm"},
						},
					}},
				},
			})

			Expect(deployment.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Volumes[0].Name).To(Equal("shared"))
			Expect(deployment.Spec.Template.Spec.Volumes[0].EmptyDir).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Volumes[0].ConfigMap).ToNot(BeNil())
		})

		Context("PodSecurityContext", func() {
			It("should apply runAsUser and fsGroup on Kubernetes", func() {
				infrastructure.InitializeForTesting(infrastructure.Kubernetes)

				Deployment.ApplyOverrides(deployment, &chev2.Deployment{
					SecurityContext: &chev2.PodSecurityContext{
						RunAsUser: new(int64(1001)),
						FsGroup:   new(int64(1001)),
					},
				})

				Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsUser).To(Equal(new(int64(1001))))
				Expect(deployment.Spec.Template.Spec.SecurityContext.FSGroup).To(Equal(new(int64(1001))))
			})

			It("should not apply pod security context on OpenShift", func() {
				infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

				Deployment.ApplyOverrides(deployment, &chev2.Deployment{
					SecurityContext: &chev2.PodSecurityContext{
						RunAsUser: new(int64(1001)),
						FsGroup:   new(int64(1001)),
					},
				})

				Expect(deployment.Spec.Template.Spec.SecurityContext).To(BeNil())
			})
		})
	})

	Describe("TrimCPULimitIfNoLimitRange", func() {
		It("should remove CPU limits when no LimitRange exists", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().Build()
			deployment := newDeployment("test",
				corev1.Container{
					Name: "test",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("250m"),
						},
					},
				},
				corev1.Container{
					Name: "gateway",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
				},
			)

			Expect(Deployment.TrimCPULimitIfNoLimitRange(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits).ToNot(HaveKey(corev1.ResourceCPU))
			Expect(deployment.Spec.Template.Spec.Containers[1].Resources.Limits).ToNot(HaveKey(corev1.ResourceCPU))
		})

		It("should keep CPU limits when LimitRange exists", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.LimitRange{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "eclipse-che",
					},
				},
			).Build()

			deployment := newDeployment("test",
				corev1.Container{
					Name: "test",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("250m"),
						},
					},
				},
			)

			Expect(Deployment.TrimCPULimitIfNoLimitRange(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("250m"))
		})
	})

	Describe("Sync", func() {
		It("should sync deployment and update env variables", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().Build()
			deployment := newDeployment("test")
			deployment.Spec.Replicas = new(int32(0))
			deployment.Spec.Template.Spec.Containers[0].Name = "test"
			deployment.Spec.Template.Spec.Containers[0].Image = "test-image"
			deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
			}

			By("syncing the deployment initially")
			Expect(Deployment.Sync(cheCtx, deployment)).To(Succeed())

			By("changing env variables and syncing again")
			deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "test-name",
					Value: "test-value",
				},
			}
			Expect(Deployment.Sync(cheCtx, deployment)).To(Succeed())

			By("syncing twice to be sure update done correctly")
			Expect(Deployment.Sync(cheCtx, deployment)).To(Succeed())

			actual := &appsv1.Deployment{}
			Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)).To(Succeed())

			value := utils.GetEnvByName("test-name", actual.Spec.Template.Spec.Containers[0].Env)
			Expect(value).To(Equal("test-value"))
		})

		It("should return not ready error when deployment is not ready", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().Build()
			deployment := newDeployment("test")
			deployment.Spec.Replicas = new(int32(0))
			deployment.Spec.Template.Spec.Containers[0].Name = "test"
			deployment.Spec.Template.Spec.Containers[0].Image = "test-image"

			Expect(Deployment.Sync(cheCtx, deployment)).To(Succeed())

			By("marking the deployment as not ready")
			actual := &appsv1.Deployment{}
			Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)).To(Succeed())
			actual.Status.UnavailableReplicas = 1
			Expect(cheCtx.ClusterAPI.Client.Status().Update(cheCtx.Context, actual)).To(Succeed())

			By("syncing the unchanged deployment")
			err := Deployment.Sync(cheCtx, deployment)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&cheerrors.NotReadyError{}))
		})
	})

	Describe("Delete", func() {
		It("should delete deployment", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().Build()
			deployment := newDeployment("test")
			deployment.Spec.Replicas = new(int32(0))
			deployment.Spec.Template.Spec.Containers[0].Name = "test"
			deployment.Spec.Template.Spec.Containers[0].Image = "test-image"

			Expect(Deployment.Sync(cheCtx, deployment)).To(Succeed())

			Expect(Deployment.Delete(cheCtx, deployment.Name, deployment.Namespace)).To(Succeed())

			found, err := Deployment.Get(cheCtx, deployment.Name, deployment.Namespace, &appsv1.Deployment{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("mountSecrets", func() {
		It("should mount several secrets sorted by name", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b-secret",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
						Annotations: map[string]string{constants.CheEclipseOrgMountAs: "env"},
					},
					Data: map[string][]byte{"key": []byte("b-data")},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "a-secret",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
						Annotations: map[string]string{constants.CheEclipseOrgMountAs: "env"},
					},
					Data: map[string][]byte{"key": []byte("a-data")},
				},
			).Build()

			Expect(Deployment.mountSecrets(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(2))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name).To(Equal("a-secret"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].ValueFrom.SecretKeyRef.Name).To(Equal("b-secret"))
		})

		It("should mount secret as file", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-volume",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "file",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
					},
					Data: map[string][]byte{
						"key": []byte("key-data"),
					},
				},
			).Build()

			Expect(Deployment.mountSecrets(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name:         "test-volume",
					VolumeSource: VolumeSource.FromSecret("test-volume"),
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test-path",
				},
			}))
		})

		It("should mount secret as subpath", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-volume",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "subpath",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
					},
					Data: map[string][]byte{
						"key": []byte("key-data"),
					},
				},
			).Build()

			Expect(Deployment.mountSecrets(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name:         "test-volume",
					VolumeSource: VolumeSource.FromSecret("test-volume"),
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test-path/key",
					SubPath:   "key",
				},
			}))
		})

		It("should mount secret as env variable", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs: "env",
							constants.CheEclipseOrgEnvName: "ENV_A",
						},
					},
					Data: map[string][]byte{
						"a": []byte("a-data"),
					},
				},
			).Build()

			Expect(Deployment.mountSecrets(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(Equal([]corev1.EnvVar{
				{
					Name: "ENV_A",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "a",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
			}))
		})

		It("should mount several env variables", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:          "env",
							constants.CheEclipseOrg + "/a_env-name": "ENV_A",
							constants.CheEclipseOrg + "/b_env-name": "ENV_B",
							constants.CheEclipseOrg + "/c_env-name": "ENV_C",
						},
					},
					Data: map[string][]byte{
						"b": []byte("a-data"),
						"a": []byte("b-data"),
						"c": []byte("c-data"),
					},
				},
			).Build()

			Expect(Deployment.mountSecrets(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(Equal([]corev1.EnvVar{
				{
					Name: "ENV_A",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "a",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
				{
					Name: "ENV_B",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "b",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
				{
					Name: "ENV_C",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "c",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
			}))
		})

		It("should sort subpath volume mounts", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "eclipse-che",
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "subpath",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret",
						},
					},
					Data: map[string][]byte{
						"e": []byte("e"), "c": []byte("c"), "a": []byte("a"), "d": []byte("d"), "b": []byte("b"),
					},
				},
			).Build()

			expectedSubPaths := []string{"a", "b", "c", "d", "e"}

			// map iteration order is random, so repeat several times
			for range 10 {
				deployment := newDeployment("che")
				Expect(Deployment.mountSecrets(cheCtx, deployment)).To(Succeed())

				subPaths := []string{}
				for _, vm := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
					subPaths = append(subPaths, vm.SubPath)
				}
				Expect(subPaths).To(Equal(expectedSubPaths))
			}
		})
	})

	Describe("mountConfigMaps", func() {
		It("should mount several configmaps sorted by name", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b-configmap",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
						Annotations: map[string]string{constants.CheEclipseOrgMountAs: "env"},
					},
					Data: map[string]string{"key": "b-data"},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "a-configmap",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
						Annotations: map[string]string{constants.CheEclipseOrgMountAs: "env"},
					},
					Data: map[string]string{"key": "a-data"},
				},
			).Build()

			Expect(Deployment.mountConfigMaps(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(2))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.ConfigMapKeyRef.Name).To(Equal("a-configmap"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].ValueFrom.ConfigMapKeyRef.Name).To(Equal("b-configmap"))
		})

		It("should mount configmap as file", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-volume",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "file",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
					},
					Data: map[string]string{
						"key": "key-data",
					},
				},
			).Build()

			Expect(Deployment.mountConfigMaps(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name:         "test-volume",
					VolumeSource: VolumeSource.FromConfigMap("test-volume"),
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test-path",
				},
			}))
		})

		It("should mount configmap as subpath", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-volume",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "subpath",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
					},
					Data: map[string]string{
						"key": "key-data",
					},
				},
			).Build()

			Expect(Deployment.mountConfigMaps(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name:         "test-volume",
					VolumeSource: VolumeSource.FromConfigMap("test-volume"),
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test-path/key",
					SubPath:   "key",
				},
			}))
		})

		It("should mount configmap as env variable", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs: "env",
							constants.CheEclipseOrgEnvName: "ENV_A",
						},
					},
					Data: map[string]string{
						"a": "a-data",
					},
				},
			).Build()

			Expect(Deployment.mountConfigMaps(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(Equal([]corev1.EnvVar{
				{
					Name: "ENV_A",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: "a",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
			}))
		})

		It("should mount several env variables", func() {
			test.StartOn(infrastructure.Kubernetes)

			deployment := newDeployment("che")
			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:          "env",
							constants.CheEclipseOrg + "/a_env-name": "ENV_A",
							constants.CheEclipseOrg + "/b_env-name": "ENV_B",
							constants.CheEclipseOrg + "/c_env-name": "ENV_C",
						},
					},
					Data: map[string]string{
						"b": "a-data",
						"a": "b-data",
						"c": "c-data",
					},
				},
			).Build()

			Expect(Deployment.mountConfigMaps(cheCtx, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(Equal([]corev1.EnvVar{
				{
					Name: "ENV_A",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: "a",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
				{
					Name: "ENV_B",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: "b",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
				{
					Name: "ENV_C",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: "c",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-envs",
							},
						},
					},
				},
			}))
		})

		It("should sort subpath volume mounts", func() {
			test.StartOn(infrastructure.Kubernetes)

			cheCtx := test.NewCheContextBuilder().WithObjects(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "eclipse-che",
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "subpath",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap",
						},
					},
					Data: map[string]string{
						"e": "e", "c": "c", "a": "a", "d": "d", "b": "b",
					},
				},
			).Build()

			expectedSubPaths := []string{"a", "b", "c", "d", "e"}

			// map iteration order is random, so repeat several times
			for range 10 {
				deployment := newDeployment("che")
				Expect(Deployment.mountConfigMaps(cheCtx, deployment)).To(Succeed())

				subPaths := []string{}
				for _, vm := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
					subPaths = append(subPaths, vm.SubPath)
				}
				Expect(subPaths).To(Equal(expectedSubPaths))
			}
		})
	})

	Describe("mountable.envName", func() {
		It("should use key-specific annotation", func() {
			m := mountable{
				annotations: map[string]string{
					constants.CheEclipseOrg + "/key_env-name": "CUSTOM_NAME",
				},
				dataKeys: []string{"key", "other"},
			}
			Expect(m.envName("key")).To(Equal("CUSTOM_NAME"))
		})

		It("should use fallback annotation for a single data key", func() {
			m := mountable{
				annotations: map[string]string{
					constants.CheEclipseOrgEnvName: "ENV_NAME",
				},
				dataKeys: []string{"key"},
			}
			Expect(m.envName("key")).To(Equal("ENV_NAME"))
		})

		It("should use the data key when there are several keys and no key-specific annotation", func() {
			m := mountable{
				annotations: map[string]string{
					constants.CheEclipseOrgEnvName: "ENV_NAME",
				},
				dataKeys: []string{"key", "other"},
			}
			Expect(m.envName("key")).To(Equal("key"))
		})

		It("should use the data key for a single key without annotations", func() {
			m := mountable{
				annotations: map[string]string{},
				dataKeys:    []string{"key"},
			}
			Expect(m.envName("key")).To(Equal("key"))
		})
	})
})
