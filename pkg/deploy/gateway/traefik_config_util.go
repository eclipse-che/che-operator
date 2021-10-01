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

func CreateCommonTraefikConfig(componentName string, rule string, priority int, serviceAddr string) *TraefikConfig {
	return &TraefikConfig{
		HTTP: TraefikConfigHTTP{
			Routers: map[string]*TraefikConfigRouter{
				componentName: {
					Rule:        rule,
					Service:     componentName,
					Middlewares: []string{},
					Priority:    priority,
				},
			},
			Services: map[string]*TraefikConfigService{
				componentName: {
					LoadBalancer: TraefikConfigLoadbalancer{
						Servers: []TraefikConfigLoadbalancerServer{
							{
								URL: serviceAddr,
							},
						},
					},
				},
			},
			Middlewares: map[string]*TraefikConfigMiddleware{},
		},
	}
}

func (cfg *TraefikConfig) AddStripPrefix(componentName string, prefixes []string) {
	middlewareName := componentName + "-strip-prefix"
	cfg.HTTP.Routers[componentName].Middlewares = append(cfg.HTTP.Routers[componentName].Middlewares, middlewareName)
	cfg.HTTP.Middlewares[middlewareName] = &TraefikConfigMiddleware{
		StripPrefix: &TraefikConfigStripPrefix{
			Prefixes: prefixes,
		},
	}
}

func (cfg *TraefikConfig) AddAuthHeaderRewrite(componentName string) {
	middlewareName := componentName + "-header-rewrite"
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
