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

func AddStripPrefix(cfg *TraefikConfig, componentName string, prefixes []string) {
	middlewareName := componentName + "-strip-prefix"
	cfg.HTTP.Routers[componentName].Middlewares = append(cfg.HTTP.Routers[componentName].Middlewares, middlewareName)
	cfg.HTTP.Middlewares[middlewareName] = &TraefikConfigMiddleware{
		StripPrefix: &TraefikConfigStripPrefix{
			Prefixes: prefixes,
		},
	}
}

func AddAuthHeaderRewrite(cfg *TraefikConfig, componentName string) {
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
