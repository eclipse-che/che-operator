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

package proxyconfig

import (
	"os"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProxyConfigReconciler Reconcile", func() {
	var reconciler *ProxyConfigReconciler

	BeforeEach(func() {
		reconciler = NewReconciler()

		Expect(os.Setenv("KUBERNETES_SERVICE_HOST", "")).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv("KUBERNETES_SERVICE_HOST")).To(Succeed())
		})
	})

	Describe("on Kubernetes", func() {
		BeforeEach(func() {
			infrastructure.InitializeForTesting(infrastructure.Kubernetes)
		})

		It("should set empty proxy when nothing is configured", func() {
			cheCtx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							Proxy: nil,
						},
					},
				},
			}).Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeFalse())
		})

		It("should use proxy from CheCluster CR and merge in-cluster service host into non proxy hosts", func() {
			Expect(os.Setenv("KUBERNETES_SERVICE_HOST", "10.10.10.10")).To(Succeed())
			DeferCleanup(func() {
				Expect(os.Setenv("KUBERNETES_SERVICE_HOST", "")).To(Succeed())
			})

			cheCtx := test.NewCtxBuilder().
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: &chev2.Proxy{
									Url:           "http://myproxy.com",
									Port:          "3128",
									NonProxyHosts: []string{"localhost"},
								},
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeTrue())
			Expect(cheCtx.Proxy.HttpProxy).To(Equal("http://myproxy.com:3128"))
			Expect(cheCtx.Proxy.HttpsProxy).To(Equal("http://myproxy.com:3128"))
			Expect(cheCtx.Proxy.NoProxy).To(Equal(".svc,10.10.10.10,localhost"))
		})
	})

	Describe("on OpenShift", func() {
		BeforeEach(func() {
			infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)
		})

		It("should set empty proxy when nothing is configured", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.ProxySpec{
						TrustedCA: configv1.ConfigMapNameReference{
							Name: "",
						},
					},
					Status: configv1.ProxyStatus{},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: nil,
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeFalse())
		})

		It("should always store trusted CA config map name even when proxy is not present", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.ProxySpec{
						TrustedCA: configv1.ConfigMapNameReference{
							Name: "additional-cluster-ca-bundle",
						},
					},
					Status: configv1.ProxyStatus{},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: nil,
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeFalse())
			Expect(cheCtx.Proxy.TrustedCAMapName).To(Equal("additional-cluster-ca-bundle"))
		})

		It("should use cluster wide proxy configuration", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.ProxySpec{
						TrustedCA: configv1.ConfigMapNameReference{
							Name: "additional-cluster-ca-bundle",
						},
					},
					Status: configv1.ProxyStatus{
						HTTPProxy:  "http://myproxy.com:3128",
						HTTPSProxy: "http://myproxy.com:3128",
						NoProxy:    "host1",
					},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: nil,
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeTrue())
			Expect(cheCtx.Proxy.HttpProxy).To(Equal("http://myproxy.com:3128"))
			Expect(cheCtx.Proxy.HttpsProxy).To(Equal("http://myproxy.com:3128"))
			Expect(cheCtx.Proxy.NoProxy).To(Equal(".svc,host1"))
			Expect(cheCtx.Proxy.TrustedCAMapName).To(Equal("additional-cluster-ca-bundle"))
		})

		It("should use cluster wide HTTPS-only proxy and keep HTTP fields empty", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.ProxySpec{
						TrustedCA: configv1.ConfigMapNameReference{
							Name: "additional-cluster-ca-bundle",
						},
					},
					Status: configv1.ProxyStatus{
						HTTPSProxy: "https://myproxy.com:3128",
						NoProxy:    "host1",
					},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: nil,
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeTrue())
			Expect(cheCtx.Proxy.HttpProxy).To(BeEmpty())
			Expect(cheCtx.Proxy.HttpsProxy).To(Equal("https://myproxy.com:3128"))
			Expect(cheCtx.Proxy.HttpsHost).To(Equal("myproxy.com"))
			Expect(cheCtx.Proxy.HttpsPort).To(Equal("3128"))
			// non proxy hosts are merged even for an HTTPS-only cluster proxy
			Expect(cheCtx.Proxy.NoProxy).To(Equal(".svc,host1"))
			Expect(cheCtx.Proxy.TrustedCAMapName).To(Equal("additional-cluster-ca-bundle"))
			// che-server JAVA_OPTS must not contain empty HTTP proxy flags
			Expect(cheCtx.Proxy.AsJavaOpts()).ToNot(ContainSubstring("http.proxyHost"))
		})

		It("should override cluster wide proxy with CheCluster CR proxy", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.ProxySpec{
						TrustedCA: configv1.ConfigMapNameReference{
							Name: "additional-cluster-ca-bundle",
						},
					},
					Status: configv1.ProxyStatus{
						HTTPProxy:  "http://cluster-proxy.com:3128",
						HTTPSProxy: "http://cluster-proxy.com:3128",
						NoProxy:    "host1",
					},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: &chev2.Proxy{
									Url:           "http://che-proxy.com",
									Port:          "8080",
									NonProxyHosts: []string{"host2"},
								},
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeTrue())
			Expect(cheCtx.Proxy.HttpProxy).To(Equal("http://che-proxy.com:8080"))
			Expect(cheCtx.Proxy.HttpsProxy).To(Equal("http://che-proxy.com:8080"))
			// non-proxy hosts from CR and cluster config are merged
			Expect(cheCtx.Proxy.NoProxy).To(Equal(".svc,host1,host2"))
			Expect(cheCtx.Proxy.TrustedCAMapName).To(Equal("additional-cluster-ca-bundle"))
		})

		It("should use cluster wide proxy when CR has non proxy hosts only", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Status: configv1.ProxyStatus{
						HTTPProxy:  "http://cluster-proxy.com:3128",
						HTTPSProxy: "http://cluster-proxy.com:3128",
						NoProxy:    "host1",
					},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: &chev2.Proxy{
									NonProxyHosts: []string{"host2"},
								},
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			// the cluster wide proxy must not be overridden by an empty CR url
			Expect(cheCtx.Proxy.HttpProxy).To(Equal("http://cluster-proxy.com:3128"))
			Expect(cheCtx.Proxy.HttpsProxy).To(Equal("http://cluster-proxy.com:3128"))
			// CR non proxy hosts still extend the cluster bypass list
			Expect(cheCtx.Proxy.NoProxy).To(Equal(".svc,host1,host2"))
		})

		It("should merge non proxy hosts from CheCluster CR when cluster proxy is not configured", func() {
			cheCtx := test.NewCtxBuilder().
				WithObjects(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.ProxySpec{
						TrustedCA: configv1.ConfigMapNameReference{
							Name: "",
						},
					},
					Status: configv1.ProxyStatus{},
				}).
				WithCheCluster(&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Components: chev2.CheClusterComponents{
							CheServer: chev2.CheServer{
								Proxy: &chev2.Proxy{
									Url:           "http://che-proxy.com",
									NonProxyHosts: []string{"host2"},
								},
							},
						},
					},
				}).
				Build()

			result, done, err := reconciler.Reconcile(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cheCtx.Proxy.IsPresent()).To(BeTrue())
			Expect(cheCtx.Proxy.NoProxy).To(Equal(".svc,host2"))
		})
	})
})
