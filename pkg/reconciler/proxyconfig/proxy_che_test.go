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
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/cheproxy"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("buildFromCheConfig", func() {
	var cheCtx *test.CheContextBuild

	BeforeEach(func() {
		cheCtx = test.NewCtxBuilder()
	})

	It("should return empty proxy when che config is nil", func() {
		proxy, err := buildFromCheConfig(cheCtx.Build(), nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(Equal(&cheproxy.Proxy{}))
		Expect(proxy.IsPresent()).To(BeFalse())
	})

	It("should build proxy from url without protocol and port", func() {
		cheProxy := &chev2.Proxy{
			Url: "myproxy.com",
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.HttpProxy).To(Equal("myproxy.com"))
		Expect(proxy.HttpsProxy).To(Equal("myproxy.com"))
		Expect(proxy.HttpHost).To(Equal("myproxy.com"))
		Expect(proxy.HttpPort).To(BeEmpty())
		Expect(proxy.HttpUser).To(BeEmpty())
		Expect(proxy.HttpPassword).To(BeEmpty())
		Expect(proxy.NoProxy).To(BeEmpty())
	})

	It("should build proxy with protocol, port and non proxy hosts", func() {
		cheProxy := &chev2.Proxy{
			Url:           "https://myproxy.com",
			Port:          "3128",
			NonProxyHosts: []string{"localhost", ".example.com"},
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(Equal(&cheproxy.Proxy{
			HttpProxy:  "https://myproxy.com:3128",
			HttpHost:   "myproxy.com",
			HttpPort:   "3128",
			HttpsProxy: "https://myproxy.com:3128",
			HttpsHost:  "myproxy.com",
			HttpsPort:  "3128",
			NoProxy:    "localhost,.example.com",
		}))
	})

	It("should apply credentials from default secret", func() {
		cheCtx.WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultProxyCredentialsSecret,
				Namespace: "eclipse-che",
			},
			Data: map[string][]byte{
				"user":     []byte(("user")),
				"password": []byte(("password")),
			},
		})
		cheProxy := &chev2.Proxy{
			Url:  "http://myproxy.com",
			Port: "3128",
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(Equal(&cheproxy.Proxy{
			HttpProxy:     "http://user:password@myproxy.com:3128",
			HttpUser:      "user",
			HttpPassword:  "password",
			HttpHost:      "myproxy.com",
			HttpPort:      "3128",
			HttpsProxy:    "http://user:password@myproxy.com:3128",
			HttpsUser:     "user",
			HttpsPassword: "password",
			HttpsHost:     "myproxy.com",
			HttpsPort:     "3128",
		}))
	})

	It("should url-encode credentials with reserved characters in the proxy URL", func() {
		cheCtx.WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultProxyCredentialsSecret,
				Namespace: "eclipse-che",
			},
			Data: map[string][]byte{
				"user":     []byte(("us@r")),
				"password": []byte(("p#ss:w/rd")),
			},
		})
		cheProxy := &chev2.Proxy{
			Url:  "http://myproxy.com",
			Port: "3128",
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		// credentials are percent-encoded in the proxy URL...
		Expect(proxy.HttpProxy).To(Equal("http://us%40r:p%23ss%3Aw%2Frd@myproxy.com:3128"))
		Expect(proxy.HttpsProxy).To(Equal("http://us%40r:p%23ss%3Aw%2Frd@myproxy.com:3128"))
		// ...but stay raw in the dedicated fields, e.g. for AsJavaOpts
		Expect(proxy.HttpUser).To(Equal("us@r"))
		Expect(proxy.HttpPassword).To(Equal("p#ss:w/rd"))
	})

	It("should apply credentials from custom secret", func() {
		cheCtx.WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-proxy-credentials",
				Namespace: "eclipse-che",
			},
			Data: map[string][]byte{
				"user":     []byte(("user")),
				"password": []byte(("password")),
			},
		})
		cheProxy := &chev2.Proxy{
			Url:                   "myproxy.com",
			CredentialsSecretName: "custom-proxy-credentials",
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.HttpProxy).To(Equal("user:password@myproxy.com"))
		Expect(proxy.HttpUser).To(Equal("user"))
		Expect(proxy.HttpPassword).To(Equal("password"))
	})

	It("should not apply credentials when secret does not exist", func() {
		cheProxy := &chev2.Proxy{
			Url: "myproxy.com",
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.HttpProxy).To(Equal("myproxy.com"))
		Expect(proxy.HttpUser).To(BeEmpty())
		Expect(proxy.HttpPassword).To(BeEmpty())
	})

	It("should keep proxy URLs empty when url is not set, even with port and credentials", func() {
		cheCtx.WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultProxyCredentialsSecret,
				Namespace: "eclipse-che",
			},
			Data: map[string][]byte{
				"user":     []byte(("user")),
				"password": []byte(("password")),
			},
		})
		cheProxy := &chev2.Proxy{
			Port:          "3128",
			NonProxyHosts: []string{"host1"},
		}

		proxy, err := buildFromCheConfig(cheCtx.Build(), cheProxy)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.IsPresent()).To(BeFalse())
		Expect(proxy.HttpProxy).To(BeEmpty())
		Expect(proxy.HttpsProxy).To(BeEmpty())
		Expect(proxy.HttpHost).To(BeEmpty())
		Expect(proxy.HttpPort).To(BeEmpty())
		// non proxy hosts are preserved so they can extend the cluster bypass list
		Expect(proxy.NoProxy).To(Equal("host1"))
	})
})
