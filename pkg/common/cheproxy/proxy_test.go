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

package cheproxy

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/http/httpproxy"
	corev1 "k8s.io/api/core/v1"
)

func TestProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Che Proxy Suite")
}

func newTestProxy() *Proxy {
	return &Proxy{
		HttpProxy:    "http://user:password@myproxy.com:1234",
		HttpUser:     "user",
		HttpPassword: "password",
		HttpHost:     "myproxy.com",
		HttpPort:     "1234",

		HttpsProxy:    "https://user:password@myproxy.com:1234",
		HttpsUser:     "user",
		HttpsPassword: "password",
		HttpsHost:     "myproxy.com",
		HttpsPort:     "1234",

		NoProxy: "localhost,myhost.com",
	}
}

var _ = Describe("Proxy", func() {
	Describe("IsPresent", func() {
		It("should return false for nil proxy", func() {
			Expect((*Proxy)(nil).IsPresent()).To(BeFalse())
		})

		It("should return false for empty proxy", func() {
			Expect((&Proxy{}).IsPresent()).To(BeFalse())
		})

		It("should return true when only HTTP proxy is set", func() {
			Expect((&Proxy{HttpProxy: "http://proxy:3128"}).IsPresent()).To(BeTrue())
		})

		It("should return true when only HTTPS proxy is set", func() {
			Expect((&Proxy{HttpsProxy: "http://proxy:3128"}).IsPresent()).To(BeTrue())
		})
	})

	Describe("AsEnvVars", func() {
		It("should return empty env vars for nil proxy", func() {
			Expect((*Proxy)(nil).AsEnvVars()).To(BeEmpty())
		})

		It("should return empty env vars for non-present proxy", func() {
			Expect((&Proxy{NoProxy: "localhost"}).AsEnvVars()).To(BeEmpty())
		})

		It("should return lower and upper case proxy env vars", func() {
			proxy := newTestProxy()
			envVars := proxy.AsEnvVars()

			Expect(envVars).To(ContainElement(corev1.EnvVar{Name: "HTTP_PROXY", Value: proxy.HttpProxy}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{Name: "HTTPS_PROXY", Value: proxy.HttpsProxy}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{Name: "http_proxy", Value: proxy.HttpProxy}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{Name: "https_proxy", Value: proxy.HttpsProxy}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{Name: "NO_PROXY", Value: proxy.NoProxy}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{Name: "no_proxy", Value: proxy.NoProxy}))
		})
	})

	Describe("AsMap", func() {
		It("should return empty map for nil proxy", func() {
			Expect((*Proxy)(nil).AsMap()).To(BeEmpty())
		})

		It("should return empty map for non-present proxy", func() {
			Expect((&Proxy{NoProxy: "localhost"}).AsMap()).To(BeEmpty())
		})

		It("should return map with lower and upper case keys", func() {
			proxy := newTestProxy()

			Expect(proxy.AsMap()).To(Equal(map[string]string{
				"http_proxy":  proxy.HttpProxy,
				"https_proxy": proxy.HttpsProxy,
				"no_proxy":    proxy.NoProxy,
				"HTTP_PROXY":  proxy.HttpProxy,
				"HTTPS_PROXY": proxy.HttpsProxy,
				"NO_PROXY":    proxy.NoProxy,
			}))
		})
	})

	Describe("AsJavaOpts", func() {
		It("should return empty string for nil proxy", func() {
			Expect((*Proxy)(nil).AsJavaOpts()).To(BeEmpty())
		})

		It("should return empty string for non-present proxy", func() {
			Expect((&Proxy{NoProxy: "localhost"}).AsJavaOpts()).To(BeEmpty())
		})

		It("should build java opts with authentication", func() {
			javaOpts := newTestProxy().AsJavaOpts()
			expectedJavaOpts := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
				"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='localhost|myhost.com' -Dhttp.proxyUser=user " +
				"-Dhttps.proxyUser=user -Dhttp.proxyPassword=password -Dhttps.proxyPassword=password " +
				"-Djdk.http.auth.tunneling.disabledSchemes= -Djdk.http.auth.proxying.disabledSchemes="
			Expect(javaOpts).To(Equal(expectedJavaOpts))
		})

		It("should build java opts without authentication", func() {
			proxy := newTestProxy()
			proxy.HttpUser = ""
			proxy.HttpPassword = ""

			javaOpts := proxy.AsJavaOpts()
			expectedJavaOpts := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
				"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='localhost|myhost.com'"
			Expect(javaOpts).To(Equal(expectedJavaOpts))
		})

		It("should handle wildcards and spaces in NoProxy", func() {
			proxy := newTestProxy()
			proxy.HttpUser = ""
			proxy.HttpPassword = ""
			proxy.NoProxy = ".example.com,localhost, *.wildcard.domain.com ,myhost.com , .wildcard.net , 127.* "

			javaOpts := proxy.AsJavaOpts()
			expectedJavaOpts := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
				"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='*.example.com|localhost|*.wildcard.domain.com|myhost.com|*.wildcard.net|127.*'"
			Expect(javaOpts).To(Equal(expectedJavaOpts))
		})

		It("should prefix NoProxy starting with a dot with a wildcard", func() {
			proxy := newTestProxy()
			proxy.NoProxy = ".example.com"

			Expect(proxy.AsJavaOpts()).To(ContainSubstring("-Dhttp.nonProxyHosts='*.example.com'"))
		})

		It("should produce empty nonProxyHosts when NoProxy is not set", func() {
			proxy := newTestProxy()
			proxy.NoProxy = ""

			Expect(proxy.AsJavaOpts()).To(ContainSubstring("-Dhttp.nonProxyHosts=''"))
		})

		It("should strip URL scheme from proxy hosts", func() {
			proxy := newTestProxy()
			proxy.HttpHost = "http://http-proxy.com"
			proxy.HttpsHost = "https://https-proxy.com"

			javaOpts := proxy.AsJavaOpts()
			Expect(javaOpts).To(ContainSubstring("-Dhttp.proxyHost=http-proxy.com"))
			Expect(javaOpts).To(ContainSubstring("-Dhttps.proxyHost=https-proxy.com"))
		})
	})

	Describe("AsHttpProxyConfig", func() {
		It("should return empty config for empty proxy", func() {
			Expect((&Proxy{}).AsHttpProxyConfig()).To(Equal(&httpproxy.Config{}))
		})

		It("should return config with proxy fields", func() {
			proxy := newTestProxy()

			config := proxy.AsHttpProxyConfig()
			Expect(config.HTTPProxy).To(Equal(proxy.HttpProxy))
			Expect(config.HTTPSProxy).To(Equal(proxy.HttpsProxy))
			Expect(config.NoProxy).To(Equal(proxy.NoProxy))
		})
	})
})
