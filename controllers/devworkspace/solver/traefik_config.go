package solver

// A representation of the Traefik config as we need it. This is in no way complete but can be used for the purposes we need it for.
type traefikConfig struct {
	HTTP traefikConfigHTTP `json:"http"`
}

type traefikConfigHTTP struct {
	Routers     map[string]traefikConfigRouter     `json:"routers"`
	Services    map[string]traefikConfigService    `json:"services"`
	Middlewares map[string]traefikConfigMiddleware `json:"middlewares"`
}

type traefikConfigRouter struct {
	Rule        string   `json:"rule"`
	Service     string   `json:"service"`
	Middlewares []string `json:"middlewares"`
	Priority    int      `json:"priority"`
}

type traefikConfigService struct {
	LoadBalancer traefikConfigLoadbalancer `json:"loadBalancer"`
}

type traefikConfigMiddleware struct {
	StripPrefix traefikConfigStripPrefix `json:"stripPrefix"`
}

type traefikConfigLoadbalancer struct {
	Servers []traefikConfigLoadbalancerServer `json:"servers"`
}

type traefikConfigLoadbalancerServer struct {
	URL string `json:"url"`
}

type traefikConfigStripPrefix struct {
	Prefixes []string `json:"prefixes"`
}
