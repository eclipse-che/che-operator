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

const (
	StripPrefixMiddlewareSuffix   = "-strip-prefix"
	HeaderRewriteMiddlewareSuffix = "-header-rewrite"
	AuthMiddlewareSuffix          = "-auth"
)

func CreateEmptyTraefikConfig() *TraefikConfig {
	return &TraefikConfig{
		HTTP: TraefikConfigHTTP{
			Routers:     map[string]*TraefikConfigRouter{},
			Services:    map[string]*TraefikConfigService{},
			Middlewares: map[string]*TraefikConfigMiddleware{},
		},
	}
}

func CreateCommonTraefikConfig(componentName string, rule string, priority int, serviceAddr string, stripPrefixes []string) *TraefikConfig {
	cfg := CreateEmptyTraefikConfig()
	cfg.AddComponent(componentName, rule, priority, serviceAddr, stripPrefixes)
	return cfg
}

func (cfg *TraefikConfig) AddComponent(componentName string, rule string, priority int, serviceAddr string, stripPrefixes []string) {
	cfg.HTTP.Routers[componentName] = &TraefikConfigRouter{
		Rule:        rule,
		Service:     componentName,
		Middlewares: []string{},
		Priority:    priority,
	}
	cfg.HTTP.Services[componentName] = &TraefikConfigService{
		LoadBalancer: TraefikConfigLoadbalancer{
			Servers: []TraefikConfigLoadbalancerServer{
				{
					URL: serviceAddr,
				},
			},
		},
	}
	if len(stripPrefixes) > 0 {
		cfg.AddStripPrefix(componentName, stripPrefixes)
	}
}

func (cfg *TraefikConfig) AddStripPrefix(componentName string, prefixes []string) {
	middlewareName := componentName + StripPrefixMiddlewareSuffix
	cfg.HTTP.Routers[componentName].Middlewares = append(cfg.HTTP.Routers[componentName].Middlewares, middlewareName)
	cfg.HTTP.Middlewares[middlewareName] = &TraefikConfigMiddleware{
		StripPrefix: &TraefikConfigStripPrefix{
			Prefixes: prefixes,
		},
	}
}

func (cfg *TraefikConfig) AddAuthHeaderRewrite(componentName string) {
	middlewareName := componentName + HeaderRewriteMiddlewareSuffix
	cfg.HTTP.Routers[componentName].Middlewares = append(cfg.HTTP.Routers[componentName].Middlewares, middlewareName)
	cfg.HTTP.Middlewares[middlewareName] = &TraefikConfigMiddleware{
		Plugin: &TraefikPlugin{
			HeaderRewrite: &TraefikPluginHeaderRewrite{
				From:   "X-Forwarded-Access-Token",
				To:     "Authorization",
				Prefix: "Bearer ",
			},
		},
	}
}

func (cfg *TraefikConfig) AddOpenShiftTokenCheck(componentName string) {
	middlewareName := componentName + "-token-check"
	cfg.HTTP.Routers[componentName].Middlewares = append(cfg.HTTP.Routers[componentName].Middlewares, middlewareName)
	cfg.HTTP.Middlewares[middlewareName] = &TraefikConfigMiddleware{
		ForwardAuth: &TraefikConfigForwardAuth{
			Address:            "https://kubernetes.default.svc/apis/user.openshift.io/v1/users/~",
			TrustForwardHeader: true,
			TLS: &TraefikConfigTLS{
				InsecureSkipVerify: true,
			},
		},
	}
}

func (cfg *TraefikConfig) AddAuth(componentName string, authAddress string) {
	middlewareName := componentName + AuthMiddlewareSuffix
	cfg.HTTP.Routers[componentName].Middlewares = append(cfg.HTTP.Routers[componentName].Middlewares, middlewareName)
	cfg.HTTP.Middlewares[middlewareName] = &TraefikConfigMiddleware{
		ForwardAuth: &TraefikConfigForwardAuth{
			Address: authAddress,
		},
	}
}
