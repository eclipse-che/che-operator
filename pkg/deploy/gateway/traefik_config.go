//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package gateway

// A representation of the Traefik config as we need it. This is in no way complete but can be used for the purposes we need it for.
type TraefikConfig struct {
	HTTP TraefikConfigHTTP `json:"http"`
}

type TraefikConfigHTTP struct {
	Routers           map[string]*TraefikConfigRouter           `json:"routers"`
	Services          map[string]*TraefikConfigService          `json:"services"`
	Middlewares       map[string]*TraefikConfigMiddleware       `json:"middlewares,omitempty"`
	ServersTransports map[string]*TraefikConfigServersTransport `json:"serversTransports,omitempty"`
}

type TraefikConfigRouter struct {
	Rule        string   `json:"rule"`
	Service     string   `json:"service"`
	Middlewares []string `json:"middlewares"`
	Priority    int      `json:"priority"`
}

type TraefikConfigService struct {
	LoadBalancer TraefikConfigLoadbalancer `json:"loadBalancer"`
}

type TraefikConfigMiddleware struct {
	StripPrefix *TraefikConfigStripPrefix `json:"stripPrefix,omitempty"`
	ForwardAuth *TraefikConfigForwardAuth `json:"forwardAuth,omitempty"`
	Errors      *TraefikConfigErrors      `json:"errors,omitempty"`
	Headers     *TraefikConfigHeaders     `json:"headers,omitempty"`
	Retry       *TraefikConfigRetry       `json:"retry,omitempty"`
	Plugin      *TraefikPlugin            `json:"plugin,omitempty"`
}

type TraefikConfigServersTransport struct {
	ForwardingTimeouts *TraefikConfigForwardingTimeouts `json:"forwardingTimeouts"`
}

type TraefikConfigLoadbalancer struct {
	Servers          []TraefikConfigLoadbalancerServer `json:"servers"`
	ServersTransport string                            `json:"serversTransport,omitempty"`
}

type TraefikConfigLoadbalancerServer struct {
	URL string `json:"url"`
}

type TraefikConfigStripPrefix struct {
	Prefixes []string `json:"prefixes"`
}

type TraefikConfigForwardAuth struct {
	Address            string            `json:"address"`
	TrustForwardHeader bool              `json:"trustForwardHeader"`
	TLS                *TraefikConfigTLS `json:"tls,omitempty"`
}

type TraefikConfigErrors struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Query   string `json:"query"`
}

type TraefikConfigHeaders struct {
	CustomResponseHeaders map[string]string `json:"customResponseHeaders,omitempty"`
}

type TraefikConfigRetry struct {
	CustomResponseHeaders map[string]string `json:"customResponseHeaders,omitempty"`
	Attempts              int               `json:"attempts,omitempty"`
	InitialInterval       string            `json:"initialInterval,omitempty"`
}

type TraefikPlugin struct {
	HeaderRewrite *TraefikPluginHeaderRewrite `json:"header-rewrite,omitempty"`
}

type TraefikPluginHeaderRewrite struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Prefix string `json:"prefix"`
}

type TraefikConfigTLS struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify"`
}

type TraefikConfigForwardingTimeouts struct {
	DialTimeout string `json:"dialTimeout"`
}
