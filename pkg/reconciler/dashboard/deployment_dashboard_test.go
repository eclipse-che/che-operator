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

package dashboard

import (
	"fmt"
	"os"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Dashboard Deployment", func() {
	var (
		cheCtx     *chetypes.CheContext
		reconciler *DashboardReconciler
	)

	Context("metadata", func() {
		BeforeEach(func() {
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx = test.NewCheContextBuilder().Build()

			reconciler = NewReconciler()
		})

		It("should set name, namespace and service account", func() {
			deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(deployment.Name).To(Equal(defaults.GetCheFlavor() + "-dashboard"))
			Expect(deployment.Namespace).To(Equal("eclipse-che"))
			Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(defaults.GetCheFlavor() + "-dashboard"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal(defaults.GetCheFlavor() + "-dashboard"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(defaults.GetDashboardImage(cheCtx.CheCluster)))
		})

		It("should select pods by the template labels", func() {
			deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(deployment.Spec.Selector.MatchLabels).ToNot(BeEmpty())
			for key, value := range deployment.Spec.Selector.MatchLabels {
				Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue(key, value))
			}
		})
	})

	Context("security context", func() {
		BeforeEach(func() {
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx = test.NewCheContextBuilder().Build()

			reconciler = NewReconciler()
		})

		It("should drop all capabilities and disallow privilege escalation", func() {
			deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

			Expect(err).ToNot(HaveOccurred())

			securityContext := deployment.Spec.Template.Spec.Containers[0].SecurityContext

			Expect(securityContext).ToNot(BeNil())
			Expect(securityContext.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
			Expect(securityContext.AllowPrivilegeEscalation).To(Equal(new(false)))
		})
	})

	Context("resources", func() {
		When("no LimitRange exists", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.OpenShiftV4)

				cheCtx = test.NewCheContextBuilder().Build()

				reconciler = NewReconciler()
			})

			It("should set default resources without CPU limit", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				res := deployment.Spec.Template.Spec.Containers[0].Resources

				Expect(res.Requests.Memory().Equal(resource.MustParse(memoryRequest))).To(BeTrue())
				Expect(res.Requests.Cpu().Equal(resource.MustParse(cpuRequest))).To(BeTrue())
				Expect(res.Limits.Memory().Equal(resource.MustParse(memoryLimit))).To(BeTrue())
				Expect(res.Limits).ToNot(HaveKey(corev1.ResourceCPU))
			})
		})

		When("a LimitRange exists", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.OpenShiftV4)

				limitRange := &corev1.LimitRange{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "limit-range",
						Namespace: "eclipse-che",
					},
				}

				cheCtx = test.NewCheContextBuilder().WithObjects(limitRange).Build()

				reconciler = NewReconciler()
			})

			It("should keep the CPU limit", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				res := deployment.Spec.Template.Spec.Containers[0].Resources

				Expect(res.Limits.Cpu().Equal(resource.MustParse(cpuLimit))).To(BeTrue())
			})
		})

		When("resources are overridden in the CheCluster", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.OpenShiftV4)

				cheCluster := &chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							Dashboard: chev2.Dashboard{
								Deployment: &chev2.Deployment{
									Containers: []chev2.Container{
										{
											Name: defaults.GetCheFlavor() + "-dashboard",
											Resources: &chev2.ResourceRequirements{
												Requests: &chev2.ResourceList{
													Memory: new(resource.MustParse("150Mi")),
													Cpu:    new(resource.MustParse("150m")),
												},
												Limits: &chev2.ResourceList{
													Memory: new(resource.MustParse("250Mi")),
													Cpu:    new(resource.MustParse("250m")),
												},
											},
										},
									},
								},
							},
						},
					},
				}

				cheCtx = test.NewCheContextBuilder().WithCheCluster(cheCluster).Build()

				reconciler = NewReconciler()
			})

			It("should apply the overrides", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				res := deployment.Spec.Template.Spec.Containers[0].Resources

				Expect(res.Requests.Memory().Equal(resource.MustParse("150Mi"))).To(BeTrue())
				Expect(res.Requests.Cpu().Equal(resource.MustParse("150m"))).To(BeTrue())
				Expect(res.Limits.Memory().Equal(resource.MustParse("250Mi"))).To(BeTrue())
				Expect(res.Limits.Cpu().Equal(resource.MustParse("250m"))).To(BeTrue())
			})
		})
	})

	Context("environment variables", func() {
		When("running on OpenShift", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.OpenShiftV4)

				Expect(os.Setenv("RELATED_IMAGE_sample_image", "sample_image")).To(Succeed())
				DeferCleanup(func() { Expect(os.Unsetenv("RELATED_IMAGE_sample_image")).To(Succeed()) })

				console := &configv1.Console{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Status: configv1.ConsoleStatus{
						ConsoleURL: "https://console-openshift-console.apps.my-host/",
					},
				}

				cheCtx = test.NewCheContextBuilder().WithObjects(console).Build()

				reconciler = NewReconciler()
			})

			It("should provision common env vars", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				env := deployment.Spec.Template.Spec.Containers[0].Env

				Expect(env).To(ContainElements(
					corev1.EnvVar{Name: "CHE_HOST", Value: "https://che-host"},
					corev1.EnvVar{Name: "CHE_URL", Value: "https://che-host"},
					corev1.EnvVar{Name: "CHECLUSTER_CR_NAMESPACE", Value: "eclipse-che"},
					corev1.EnvVar{Name: "CHECLUSTER_CR_NAME", Value: "eclipse-che"},
					corev1.EnvVar{Name: "DWO_NAMESPACE", Value: "devworkspace-controller"},
					corev1.EnvVar{Name: "CHE_INTERNAL_URL", Value: "http://che-host.eclipse-che.svc:8080/api"},
					corev1.EnvVar{
						Name:  "CHE_DASHBOARD_INTERNAL_URL",
						Value: fmt.Sprintf("http://%s-dashboard.eclipse-che.svc:8080", defaults.GetCheFlavor()),
					},
					corev1.EnvVar{
						Name:  "CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT",
						Value: defaults.GetDashboardHeaderMessageText(),
					},
					corev1.EnvVar{
						Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTEDITOR",
						Value: defaults.GetDevEnvironmentsDefaultEditor(),
					},
					corev1.EnvVar{
						Name:  "CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL",
						Value: defaults.GetPluginRegistryOpenVSXURL(),
					},
					corev1.EnvVar{Name: "RELATED_IMAGE_sample_image", Value: "sample_image"},
				))
			})

			It("should provision the OpenShift console URL", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				env := deployment.Spec.Template.Spec.Containers[0].Env

				Expect(env).To(ContainElement(corev1.EnvVar{
					Name:  "OPENSHIFT_CONSOLE_URL",
					Value: "https://console-openshift-console.apps.my-host/",
				}))
			})
		})

		When("running on Kubernetes", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.Kubernetes)

				cheCtx = test.NewCheContextBuilder().Build()

				reconciler = NewReconciler()
			})

			It("should not provision the OpenShift console URL", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				env := deployment.Spec.Template.Spec.Containers[0].Env

				Expect(env).ToNot(ContainElement(HaveField("Name", "OPENSHIFT_CONSOLE_URL")))
			})
		})
	})

	Context("volumes", func() {
		customCAVolume := corev1.Volume{
			Name: customCertsVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "ca-certs-merged"},
				},
			},
		}
		customCAVolumeMount := corev1.VolumeMount{
			Name:      customCertsVolumeName,
			MountPath: customCertsMountPath,
		}

		When("the self-signed certificate Secret does not exist", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.OpenShiftV4)

				cheCtx = test.NewCheContextBuilder().Build()

				reconciler = NewReconciler()
			})

			It("should mount only the custom CA bundle", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				Expect(deployment.Spec.Template.Spec.Volumes).To(HaveExactElements(HaveField("Name", customCAVolume.Name)))
				Expect(deployment.Spec.Template.Spec.Volumes[0].ConfigMap.Name).To(Equal("ca-certs-merged"))
				Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{customCAVolumeMount}))
			})
		})

		When("the self-signed certificate Secret exists", func() {
			BeforeEach(func() {
				test.StartOn(infrastructure.OpenShiftV4)

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.SelfSignedCertificateSecretName,
						Namespace: "eclipse-che",
					},
				}

				cheCtx = test.NewCheContextBuilder().WithObjects(secret).Build()

				reconciler = NewReconciler()
			})

			It("should mount both the custom CA bundle and the self-signed certificate", func() {
				deployment, err := reconciler.getDashboardDeploymentSpec(cheCtx)

				Expect(err).ToNot(HaveOccurred())

				Expect(deployment.Spec.Template.Spec.Volumes).To(HaveExactElements(
					HaveField("Name", customCAVolume.Name),
					HaveField("Name", selfSignedCertVolumeName),
				))

				selfSignedVolume := deployment.Spec.Template.Spec.Volumes[1]
				Expect(selfSignedVolume.Secret).ToNot(BeNil())
				Expect(selfSignedVolume.Secret.SecretName).To(Equal(constants.SelfSignedCertificateSecretName))
				Expect(selfSignedVolume.Secret.Items).To(ContainElement(HaveField("Key", "ca.crt")))
				Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
					customCAVolumeMount,
					{Name: selfSignedCertVolumeName, MountPath: selfSignedCertMountPath},
				}))
			})
		})
	})

	Context("Reconcile", func() {
		BeforeEach(func() {
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx = test.NewCheContextBuilder().Build()

			reconciler = NewReconciler()
		})

		It("should create all objects", func() {
			Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

			name := defaults.GetCheFlavor() + "-dashboard"
			key := types.NamespacedName{Name: name, Namespace: "eclipse-che"}
			gatewayConfigKey := types.NamespacedName{
				Name:      gateway.GatewayConfigMapNamePrefix + name,
				Namespace: "eclipse-che",
			}

			Expect(test.ExpectFound(key, &appsv1.Deployment{})).To(Succeed())
			Expect(test.ExpectFound(key, &corev1.Service{})).To(Succeed())
			Expect(test.ExpectFound(key, &corev1.ServiceAccount{})).To(Succeed())
			Expect(test.ExpectFound(gatewayConfigKey, &corev1.ConfigMap{})).To(Succeed())
			Expect(test.ExpectFound(types.NamespacedName{Name: name}, &rbacv1.ClusterRole{})).To(Succeed())
			Expect(test.ExpectFound(types.NamespacedName{Name: name}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
		})
	})
})
