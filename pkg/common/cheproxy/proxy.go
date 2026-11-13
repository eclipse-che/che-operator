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
	"fmt"
	"strings"

	"golang.org/x/net/http/httpproxy"
	corev1 "k8s.io/api/core/v1"
)

type Proxy struct {
	// HttpProxy is the full HTTP proxy URL, e.g. "http://user:password@host:port".
	HttpProxy string
	// HttpUser is the username for HTTP proxy authentication.
	HttpUser string
	// HttpPassword is the password for HTTP proxy authentication.
	HttpPassword string
	// HttpHost is the HTTP proxy host without scheme, port, or credentials.
	HttpHost string
	// HttpPort is the HTTP proxy port.
	HttpPort string

	// HttpsProxy is the full HTTPS proxy URL, e.g. "https://user:password@host:port".
	HttpsProxy string
	// HttpsUser is the username for HTTPS proxy authentication.
	HttpsUser string
	// HttpsPassword is the password for HTTPS proxy authentication.
	HttpsPassword string
	// HttpsHost is the HTTPS proxy host without scheme, port, or credentials.
	HttpsHost string
	// HttpsPort is the HTTPS proxy port.
	HttpsPort string

	// NoProxy is a comma separated list of hosts that bypass the proxy.
	NoProxy string
	// TrustedCAMapName is the name of the ConfigMap holding the trusted CA
	// certificates to be used for proxy connections.
	TrustedCAMapName string
}

// IsPresent reports whether an HTTP or HTTPS proxy URL is configured.
// NoProxy and TrustedCAMapName alone do not count as a configured proxy.
func (p *Proxy) IsPresent() bool {
	if p == nil {
		return false
	}
	return p.HttpProxy != "" || p.HttpsProxy != ""
}

func (p *Proxy) AsEnvVars() []corev1.EnvVar {
	if !p.IsPresent() {
		return []corev1.EnvVar{}
	}

	return []corev1.EnvVar{
		{
			Name:  "http_proxy",
			Value: p.HttpProxy,
		},
		{
			Name:  "https_proxy",
			Value: p.HttpsProxy,
		},
		{
			Name:  "no_proxy",
			Value: p.NoProxy,
		},
		{
			Name:  "HTTP_PROXY",
			Value: p.HttpProxy,
		},
		{
			Name:  "HTTPS_PROXY",
			Value: p.HttpsProxy,
		},
		{
			Name:  "NO_PROXY",
			Value: p.NoProxy,
		},
	}
}

func (p *Proxy) AsMap() map[string]string {
	if !p.IsPresent() {
		return make(map[string]string)
	}

	return map[string]string{
		"HTTP_PROXY":  p.HttpProxy,
		"HTTPS_PROXY": p.HttpsProxy,
		"NO_PROXY":    p.NoProxy,
		"http_proxy":  p.HttpProxy,
		"https_proxy": p.HttpsProxy,
		"no_proxy":    p.NoProxy,
	}
}

func (p *Proxy) AsJavaOpts() string {
	if !p.IsPresent() {
		return ""
	}

	noProxy := p.NoProxy
	noProxy = strings.ReplaceAll(noProxy, " ", "")
	noProxy = strings.ReplaceAll(noProxy, ",", "|")
	noProxy = strings.ReplaceAll(noProxy, "|.", "|*.")
	if strings.HasPrefix(noProxy, ".") {
		noProxy = "*" + noProxy
	}

	opts := []string{}
	if p.HttpProxy != "" {
		opts = append(opts,
			"-Dhttp.proxyHost="+stripURLScheme(p.HttpHost),
			"-Dhttp.proxyPort="+p.HttpPort,
		)
	}

	if p.HttpsProxy != "" {
		opts = append(opts,
			"-Dhttps.proxyHost="+stripURLScheme(p.HttpsHost),
			"-Dhttps.proxyPort="+p.HttpsPort,
		)
	}

	opts = append(opts, fmt.Sprintf("-Dhttp.nonProxyHosts='%s'", noProxy))

	if p.HttpUser != "" && p.HttpPassword != "" {
		opts = append(opts,
			"-Dhttp.proxyUser="+p.HttpUser,
			"-Dhttp.proxyPassword="+p.HttpPassword,
		)
	}
	if p.HttpsUser != "" && p.HttpsPassword != "" {
		opts = append(opts,
			"-Dhttps.proxyUser="+p.HttpsUser,
			"-Dhttps.proxyPassword="+p.HttpsPassword,
		)
	}
	if p.hasCredentials() {
		opts = append(opts,
			"-Djdk.http.auth.tunneling.disabledSchemes=",
			"-Djdk.http.auth.proxying.disabledSchemes=",
		)
	}

	return " " + strings.Join(opts, " ")
}

func (p *Proxy) hasCredentials() bool {
	return (p.HttpUser != "" && p.HttpPassword != "") ||
		(p.HttpsUser != "" && p.HttpsPassword != "")
}

func (p *Proxy) AsHttpProxyConfig() *httpproxy.Config {
	if !p.IsPresent() {
		return &httpproxy.Config{}
	}

	return &httpproxy.Config{
		HTTPProxy:  p.HttpProxy,
		HTTPSProxy: p.HttpsProxy,
		NoProxy:    p.NoProxy,
	}
}

func stripURLScheme(url string) string {
	return strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
}
