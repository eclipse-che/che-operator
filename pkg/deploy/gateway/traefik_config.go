package gateway

// A representation of the Traefik config as we need it. This is in no way complete but can be used for the purposes we need it for.
type TraefikConfig struct {
	HTTP TraefikConfigHTTP `json:"http"`
}

type TraefikConfigHTTP struct {
	Routers     map[string]*TraefikConfigRouter     `json:"routers"`
	Services    map[string]*TraefikConfigService    `json:"services"`
	Middlewares map[string]*TraefikConfigMiddleware `json:"middlewares"`
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
	Plugin      *TraefikPlugin            `json:"plugin,omitempty"`
}

type TraefikConfigLoadbalancer struct {
	Servers []TraefikConfigLoadbalancerServer `json:"servers"`
}

type TraefikConfigLoadbalancerServer struct {
	URL string `json:"url"`
}

type TraefikConfigStripPrefix struct {
	Prefixes []string `json:"prefixes"`
}

type TraefikConfigForwardAuth struct {
	Address string `json:"address"`
}

type TraefikPlugin struct {
	HeaderRewrite *TraefikPluginHeaderRewrite `json:"header-rewrite,omitempty"`
}

type TraefikPluginHeaderRewrite struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Prefix string `json:"prefix"`
}
