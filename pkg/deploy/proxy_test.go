//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	"os"
	"reflect"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	expectedProxyURLWithUsernamePassword    = "https://user:password@myproxy.com:1234"
	expectedProxyURLWithoutUsernamePassword = "https://myproxy.com:1234"
	expectedNoProxy                         = "localhost,myhost.com"
)

func TestGenerateProxyJavaOptsWithUsernameAndPassword(t *testing.T) {
	proxy := &chetypes.Proxy{
		HttpProxy:    "https://user:password@myproxy.com:1234",
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

	if err := os.Setenv("KUBERNETES_SERVICE_HOST", "172.30.0.1"); err != nil {
		logrus.Errorf("Failed to set env %s", err)
	}

	javaOpts, _ := GenerateProxyJavaOpts(proxy, "")
	expectedJavaOpts := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
		"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='localhost|myhost.com' -Dhttp.proxyUser=user " +
		"-Dhttp.proxyPassword=password -Dhttps.proxyUser=user -Dhttps.proxyPassword=password " +
		"-Djdk.http.auth.tunneling.disabledSchemes= -Djdk.http.auth.proxying.disabledSchemes="
	if !reflect.DeepEqual(javaOpts, expectedJavaOpts) {
		t.Errorf("Test failed. Expected '%s' but got '%s'", expectedJavaOpts, javaOpts)

	}
}

func TestGenerateProxyJavaOptsWithoutAuthentication(t *testing.T) {
	proxy := &chetypes.Proxy{
		HttpProxy: "http://myproxy.com:1234",
		HttpHost:  "myproxy.com",
		HttpPort:  "1234",

		HttpsProxy: "https://myproxy.com:1234",
		HttpsHost:  "myproxy.com",
		HttpsPort:  "1234",

		NoProxy: "localhost,myhost.com",
	}
	javaOpts, _ := GenerateProxyJavaOpts(proxy, "test-no-proxy.com")
	expectedJavaOptsWithoutUsernamePassword := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
		"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='test-no-proxy.com'"

	if !reflect.DeepEqual(javaOpts, expectedJavaOptsWithoutUsernamePassword) {
		t.Errorf("Test failed. Expected '%s' but got '%s'", expectedJavaOptsWithoutUsernamePassword, javaOpts)
	}
}

func TestGenerateProxyJavaOptsWildcardInNonProxyHosts(t *testing.T) {
	proxy := &chetypes.Proxy{
		HttpProxy: "http://myproxy.com:1234",
		HttpHost:  "myproxy.com",
		HttpPort:  "1234",

		HttpsProxy: "https://myproxy.com:1234",
		HttpsHost:  "myproxy.com",
		HttpsPort:  "1234",

		NoProxy: ".example.com,localhost, *.wildcard.domain.com ,myhost.com , .wildcard.net , 127.* ",
	}
	javaOpts, _ := GenerateProxyJavaOpts(proxy, "")
	expectedJavaOptsWithoutUsernamePassword := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
		"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='*.example.com|localhost|*.wildcard.domain.com|myhost.com|*.wildcard.net|127.*'"

	if !reflect.DeepEqual(javaOpts, expectedJavaOptsWithoutUsernamePassword) {
		t.Errorf("Test failed. Expected '%s' but got '%s'", expectedJavaOptsWithoutUsernamePassword, javaOpts)
	}
}

func TestReadCheClusterProxyConfiguration(t *testing.T) {
	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					Proxy: &chev2.Proxy{
						Port:                  "1234",
						Url:                   "https://myproxy.com",
						NonProxyHosts:         []string{"host1", "host2"},
						CredentialsSecretName: "proxy",
					},
				},
			},
		},
	}
	proxySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy",
			Namespace: "eclipse-che",
		},
		Data: map[string][]byte{
			"user":     []byte("user"),
			"password": []byte("password"),
		},
	}

	expectedProxy := &chetypes.Proxy{
		HttpProxy:    "https://user:password@myproxy.com:1234",
		HttpUser:     "user",
		HttpPassword: "password",
		HttpHost:     "myproxy.com",
		HttpPort:     "1234",

		HttpsProxy:    "https://user:password@myproxy.com:1234",
		HttpsUser:     "user",
		HttpsPassword: "password",
		HttpsHost:     "myproxy.com",
		HttpsPort:     "1234",

		NoProxy: "host1,host2",
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{proxySecret})
	actualProxy, _ := ReadCheClusterProxyConfiguration(ctx)

	if !reflect.DeepEqual(actualProxy, expectedProxy) {
		t.Errorf("Test failed. Expected '%v', but got '%v'", expectedProxy, actualProxy)
	}
}

func TestReadCheClusterProxyConfigurationNoUser(t *testing.T) {
	checluster := &chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					Proxy: &chev2.Proxy{
						Port:          "1234",
						Url:           "https://myproxy.com",
						NonProxyHosts: []string{"host1", "host2"},
					},
				},
			},
		},
	}

	expectedProxy := &chetypes.Proxy{
		HttpProxy: "https://myproxy.com:1234",
		HttpHost:  "myproxy.com",
		HttpPort:  "1234",

		HttpsProxy: "https://myproxy.com:1234",
		HttpsHost:  "myproxy.com",
		HttpsPort:  "1234",

		NoProxy: "host1,host2",
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{})
	actualProxy, _ := ReadCheClusterProxyConfiguration(ctx)

	if !reflect.DeepEqual(actualProxy, expectedProxy) {
		t.Errorf("Test failed. Expected '%v', but got '%v'", expectedProxy, actualProxy)
	}
}

func TestReadCheClusterProxyConfigurationNoPort(t *testing.T) {
	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					Proxy: &chev2.Proxy{
						Url:           "https://myproxy.com",
						NonProxyHosts: []string{"host1", "host2"},
					},
				},
			},
		},
	}
	proxySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-credentials",
			Namespace: "eclipse-che",
		},
		Data: map[string][]byte{
			"user":     []byte("user"),
			"password": []byte("password"),
		},
	}

	expectedProxy := &chetypes.Proxy{
		HttpProxy:    "https://user:password@myproxy.com",
		HttpUser:     "user",
		HttpPassword: "password",
		HttpHost:     "myproxy.com",

		HttpsProxy:    "https://user:password@myproxy.com",
		HttpsUser:     "user",
		HttpsPassword: "password",
		HttpsHost:     "myproxy.com",

		NoProxy: "host1,host2",
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{proxySecret})
	actualProxy, _ := ReadCheClusterProxyConfiguration(ctx)
	assert.Equal(t, actualProxy, expectedProxy)
}

func TestReadClusterWideProxyConfiguration(t *testing.T) {
	clusterProxy := &configv1.Proxy{
		Status: configv1.ProxyStatus{
			HTTPProxy:  "http://user1:password1@myproxy1.com:1234",
			HTTPSProxy: "https://user2:password2@myproxy2.com:2345",
			NoProxy:    "host1,host2",
		},
	}

	expectedProxy := &chetypes.Proxy{
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
	}

	actualProxy, _ := ReadClusterWideProxyConfiguration(clusterProxy)

	if !reflect.DeepEqual(actualProxy, expectedProxy) {
		t.Errorf("Test failed. Expected '%v', but got '%v'", expectedProxy, actualProxy)
	}
}

func TestReadClusterWideProxyConfigurationNoUser(t *testing.T) {
	clusterProxy := &configv1.Proxy{
		Status: configv1.ProxyStatus{
			HTTPProxy: "http://myproxy.com:1234",
			NoProxy:   "host1,host2",
		},
	}

	expectedProxy := &chetypes.Proxy{
		HttpProxy: "http://myproxy.com:1234",
		HttpHost:  "myproxy.com",
		HttpPort:  "1234",

		HttpsProxy: "http://myproxy.com:1234",
		NoProxy:    "host1,host2",
		HttpsHost:  "myproxy.com",
		HttpsPort:  "1234",
	}

	actualProxy, _ := ReadClusterWideProxyConfiguration(clusterProxy)

	if !reflect.DeepEqual(actualProxy, expectedProxy) {
		t.Errorf("Test failed. Expected '%v', but got '%v'", expectedProxy, actualProxy)
	}
}

func TestReadClusterWideProxyConfigurationNoPort(t *testing.T) {
	clusterProxy := &configv1.Proxy{
		Status: configv1.ProxyStatus{
			HTTPProxy: "http://user:password@myproxy.com",
			NoProxy:   "host1,host2",
		},
	}

	expectedProxy := &chetypes.Proxy{
		HttpProxy:    "http://user:password@myproxy.com",
		HttpUser:     "user",
		HttpPassword: "password",
		HttpHost:     "myproxy.com",

		HttpsProxy:    "http://user:password@myproxy.com",
		HttpsUser:     "user",
		HttpsPassword: "password",
		HttpsHost:     "myproxy.com",

		NoProxy: "host1,host2",
	}

	actualProxy, _ := ReadClusterWideProxyConfiguration(clusterProxy)

	if !reflect.DeepEqual(actualProxy, expectedProxy) {
		t.Errorf("Test failed. Expected '%v', but got '%v'", expectedProxy, actualProxy)
	}
}
