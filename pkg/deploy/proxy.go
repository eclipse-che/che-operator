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
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"

	"golang.org/x/net/http/httpproxy"

	"net/http"
	"net/url"

	configv1 "github.com/openshift/api/config/v1"

	"github.com/sirupsen/logrus"
)

func ReadClusterWideProxyConfiguration(clusterProxy *configv1.Proxy) (*chetypes.Proxy, error) {
	proxy := &chetypes.Proxy{}

	// Cluster components consume the status values to configure the proxy for their component.
	proxy.HttpProxy = clusterProxy.Status.HTTPProxy
	proxy.HttpsProxy = clusterProxy.Status.HTTPSProxy
	if len(proxy.HttpsProxy) == 0 {
		proxy.HttpsProxy = proxy.HttpProxy
	}
	proxy.NoProxy = clusterProxy.Status.NoProxy
	httpProxy, err := url.Parse(proxy.HttpProxy)
	if err != nil {
		return nil, err
	}
	proxy.HttpHost = httpProxy.Hostname()
	proxy.HttpPort = httpProxy.Port()
	proxy.HttpUser = httpProxy.User.Username()
	proxy.HttpPassword, _ = httpProxy.User.Password()

	httpsProxy, err := url.Parse(proxy.HttpsProxy)
	if err != nil {
		return nil, err
	}
	proxy.HttpsHost = httpsProxy.Hostname()
	proxy.HttpsPort = httpsProxy.Port()
	proxy.HttpsUser = httpsProxy.User.Username()
	proxy.HttpsPassword, _ = httpsProxy.User.Password()
	proxy.TrustedCAMapName = clusterProxy.Spec.TrustedCA.Name

	return proxy, nil
}

func ReadCheClusterProxyConfiguration(ctx *chetypes.DeployContext) (*chetypes.Proxy, error) {
	if ctx.CheCluster.Spec.Components.CheServer.Proxy == nil {
		return &chetypes.Proxy{}, nil
	}

	proxyParts := strings.Split(ctx.CheCluster.Spec.Components.CheServer.Proxy.Url, "://")
	proxyProtocol := ""
	proxyHost := ""
	if len(proxyParts) == 1 {
		proxyProtocol = ""
		proxyHost = proxyParts[0]
	} else {
		proxyProtocol = proxyParts[0]
		proxyHost = proxyParts[1]
	}

	proxyURL := proxyHost
	if ctx.CheCluster.Spec.Components.CheServer.Proxy.Port != "" {
		proxyURL = proxyURL + ":" + ctx.CheCluster.Spec.Components.CheServer.Proxy.Port
	}

	proxyUser := ""
	proxyPassword := ""

	proxyCredentialsSecretName := utils.GetValue(ctx.CheCluster.Spec.Components.CheServer.Proxy.CredentialsSecretName, constants.DefaultProxyCredentialsSecret)
	proxyCredentialsSecret := &corev1.Secret{}
	exists, err := GetNamespacedObject(ctx, proxyCredentialsSecretName, proxyCredentialsSecret)
	if err != nil {
		return nil, err
	} else if exists {
		proxyUser = string(proxyCredentialsSecret.Data["user"])
		proxyPassword = string(proxyCredentialsSecret.Data["password"])
	}

	if len(proxyUser) > 1 && len(proxyPassword) > 1 {
		proxyURL = proxyUser + ":" + proxyPassword + "@" + proxyURL
	}

	if proxyProtocol != "" {
		proxyURL = proxyProtocol + "://" + proxyURL
	}

	return &chetypes.Proxy{
		HttpProxy:    proxyURL,
		HttpUser:     proxyUser,
		HttpHost:     proxyHost,
		HttpPort:     ctx.CheCluster.Spec.Components.CheServer.Proxy.Port,
		HttpPassword: proxyPassword,

		HttpsProxy:    proxyURL,
		HttpsUser:     proxyUser,
		HttpsHost:     proxyHost,
		HttpsPort:     ctx.CheCluster.Spec.Components.CheServer.Proxy.Port,
		HttpsPassword: proxyPassword,

		NoProxy: strings.Join(ctx.CheCluster.Spec.Components.CheServer.Proxy.NonProxyHosts, ","),
	}, nil
}

func MergeNonProxy(noProxy1 string, noProxy2 string) string {
	if noProxy1 == "" {
		return noProxy2
	} else if noProxy2 == "" {
		return noProxy1
	}

	return noProxy1 + "," + noProxy2
}

// GenerateProxyJavaOpts converts given proxy configuration into Java format.
func GenerateProxyJavaOpts(proxy *chetypes.Proxy, noProxy string) (javaOpts string, err error) {
	if noProxy == "" {
		noProxy = proxy.NoProxy
	}
	// Remove all spaces
	noProxy = strings.Replace(noProxy, " ", "", -1)
	// Replace , with |
	noProxy = strings.Replace(noProxy, ",", "|", -1)
	// Convert .domain wildcards to Java format *.domain
	noProxy = strings.Replace(noProxy, "|.", "|*.", -1)
	if strings.HasPrefix(noProxy, ".") {
		noProxy = "*" + noProxy
	}

	proxyUserPassword := ""
	if len(proxy.HttpUser) > 1 && len(proxy.HttpPassword) > 1 {
		proxyUserPassword = " -Dhttp.proxyUser=" + proxy.HttpUser + " -Dhttp.proxyPassword=" + proxy.HttpPassword +
			" -Dhttps.proxyUser=" + proxy.HttpsUser + " -Dhttps.proxyPassword=" + proxy.HttpsPassword +
			" -Djdk.http.auth.tunneling.disabledSchemes= -Djdk.http.auth.proxying.disabledSchemes="
	}

	javaOpts =
		" -Dhttp.proxyHost=" + removeProtocolPrefix(proxy.HttpHost) + " -Dhttp.proxyPort=" + proxy.HttpPort +
			" -Dhttps.proxyHost=" + removeProtocolPrefix(proxy.HttpsHost) + " -Dhttps.proxyPort=" + proxy.HttpsPort +
			" -Dhttp.nonProxyHosts='" + noProxy + "'" + proxyUserPassword
	return javaOpts, nil
}

func removeProtocolPrefix(url string) string {
	if strings.HasPrefix(url, "https://") {
		return strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		return strings.TrimPrefix(url, "http://")
	}
	return url
}

// ConfigureProxy adds existing proxy configuration into provided transport object.
func ConfigureProxy(deployContext *chetypes.DeployContext, transport *http.Transport) {
	config := httpproxy.Config{
		HTTPProxy:  deployContext.Proxy.HttpProxy,
		HTTPSProxy: deployContext.Proxy.HttpsProxy,
		NoProxy:    deployContext.Proxy.NoProxy,
	}
	proxyFunc := config.ProxyFunc()
	transport.Proxy = func(r *http.Request) (*url.URL, error) {
		theProxyURL, err := proxyFunc(r.URL)
		if err != nil {
			logrus.Warnf("Error when trying to get the proxy to access TLS endpoint URL: %s - %s", r.URL, err)
		}
		if theProxyURL != nil {
			logrus.Infof("Using proxy: %s to access TLS endpoint URL: %s", theProxyURL, r.URL)
		} else {
			logrus.Infof("Proxy isn't used to access TLS endpoint URL: %s", r.URL)
		}
		return theProxyURL, err
	}
}
