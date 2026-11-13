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
	"github.com/eclipse-che/che-operator/pkg/common/cheproxy"
	configv1 "github.com/openshift/api/config/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("buildFromClusterConfig", func() {
	It("should return empty proxy when cluster config has no proxies", func() {
		proxy, err := buildFromClusterConfig(&configv1.Proxy{})

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(Equal(&cheproxy.Proxy{}))
		Expect(proxy.IsPresent()).To(BeFalse())
	})

	It("should build proxy from HTTP proxy only", func() {
		clusterConfig := &configv1.Proxy{
			Status: configv1.ProxyStatus{
				HTTPProxy: "http://user:password@myproxy.com",
				NoProxy:   ".svc,localhost",
			},
		}

		proxy, err := buildFromClusterConfig(clusterConfig)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(Equal(&cheproxy.Proxy{
			HttpProxy:    "http://user:password@myproxy.com",
			HttpUser:     "user",
			HttpPassword: "password",
			HttpHost:     "myproxy.com",

			HttpsProxy:    "http://user:password@myproxy.com",
			HttpsUser:     "user",
			HttpsPassword: "password",
			HttpsHost:     "myproxy.com",

			NoProxy: ".svc,localhost",
		}))
	})

	It("should not build proxy from HTTPS proxy", func() {
		clusterConfig := &configv1.Proxy{
			Status: configv1.ProxyStatus{
				HTTPSProxy: "https://myproxy.com:3128",
			},
		}

		proxy, err := buildFromClusterConfig(clusterConfig)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.HttpsProxy).To(Equal("https://myproxy.com:3128"))
		Expect(proxy.HttpProxy).To(BeEmpty())
		Expect(proxy.HttpHost).To(BeEmpty())
		Expect(proxy.HttpPort).To(BeEmpty())
	})

	It("should keep distinct HTTP and HTTPS proxies", func() {
		clusterConfig := &configv1.Proxy{
			Status: configv1.ProxyStatus{
				HTTPProxy:  "http://user1:password1@myproxy1.com:1234",
				HTTPSProxy: "https://user2:password2@myproxy2.com:2345",
				NoProxy:    "host1,host2",
			},
		}

		proxy, err := buildFromClusterConfig(clusterConfig)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(Equal(&cheproxy.Proxy{
			HttpProxy:    "http://user1:password1@myproxy1.com:1234",
			HttpUser:     "user1",
			HttpPassword: "password1",
			HttpHost:     "myproxy1.com",
			HttpPort:     "1234",

			HttpsProxy:    "https://user2:password2@myproxy2.com:2345",
			HttpsUser:     "user2",
			HttpsPassword: "password2",
			HttpsHost:     "myproxy2.com",
			HttpsPort:     "2345",

			NoProxy: "host1,host2",
		}))
	})

	It("should parse trusted CA config map name", func() {
		clusterConfig := &configv1.Proxy{
			Spec: configv1.ProxySpec{
				TrustedCA: configv1.ConfigMapNameReference{
					Name: "trusted-ca",
				},
			},
			Status: configv1.ProxyStatus{
				HTTPProxy: "http://user:password@myproxy.com:3128",
			},
		}

		proxy, err := buildFromClusterConfig(clusterConfig)

		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.TrustedCAMapName).To(Equal("trusted-ca"))
	})

	It("should return error for invalid proxy URL", func() {
		clusterConfig := &configv1.Proxy{
			Status: configv1.ProxyStatus{
				HTTPProxy: "http://%zz",
			},
		}

		_, err := buildFromClusterConfig(clusterConfig)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse proxy config"))
	})
})
