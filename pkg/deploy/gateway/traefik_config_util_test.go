package gateway

import "testing"

const (
	testComponentName = "testComponentName"
	testRule          = "PathPrefix(`/test`)"
)

func TestAddStripPrefix(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
	AddStripPrefix(cfg, testComponentName, []string{"/test"})

	if len(cfg.HTTP.Routers[testComponentName].Middlewares) != 1 {
		t.Errorf("Expected 1 middleware in router but got '%d'. %+v", len(cfg.HTTP.Routers[testComponentName].Middlewares), cfg)
	}

	if len(cfg.HTTP.Middlewares) != 1 {
		t.Errorf("Expected 1 middlewares but got '%d'. %+v", len(cfg.HTTP.Middlewares), cfg)
	}

	if _, ok := cfg.HTTP.Middlewares[cfg.HTTP.Routers[testComponentName].Middlewares[0]]; !ok {
		t.Errorf("Middleware in router does not match middleware definition. %+v", cfg)
	}
}

func TestAddAuthHeaderRewrite(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
	AddAuthHeaderRewrite(cfg, testComponentName)

	if len(cfg.HTTP.Routers[testComponentName].Middlewares) != 1 {
		t.Errorf("Expected 1 middleware in router but got '%d'. %+v", len(cfg.HTTP.Routers[testComponentName].Middlewares), cfg)
	}

	if len(cfg.HTTP.Middlewares) != 1 {
		t.Errorf("Expected 1 middlewares but got '%d'. %+v", len(cfg.HTTP.Middlewares), cfg)
	}

	if _, ok := cfg.HTTP.Middlewares[cfg.HTTP.Routers[testComponentName].Middlewares[0]]; !ok {
		t.Errorf("Middleware in router does not match middleware definition. %+v", cfg)
	}
}

func TestMiddlewaresPreserveOrder(t *testing.T) {
	t.Run("strip-header", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
		AddStripPrefix(cfg, testComponentName, []string{"/test"})
		AddAuthHeaderRewrite(cfg, testComponentName)

		if cfg.HTTP.Routers[testComponentName].Middlewares[0] != testComponentName+"-strip-prefix" {
			t.Errorf("first middleware should be strip-prefix")
		}

		if cfg.HTTP.Routers[testComponentName].Middlewares[1] != testComponentName+"-header-rewrite" {
			t.Errorf("first middleware should be header-rewrite")
		}
	})

	t.Run("header-strip", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
		AddAuthHeaderRewrite(cfg, testComponentName)
		AddStripPrefix(cfg, testComponentName, []string{"/test"})

		if cfg.HTTP.Routers[testComponentName].Middlewares[0] != testComponentName+"-header-rewrite" {
			t.Errorf("first middleware should be header-rewrite")
		}

		if cfg.HTTP.Routers[testComponentName].Middlewares[1] != testComponentName+"-strip-prefix" {
			t.Errorf("first middleware should be strip-prefix")
		}
	})
}

func TestCreateCommonTraefikConfig(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")

	if len(cfg.HTTP.Routers[testComponentName].Middlewares) != 0 {
		t.Errorf("Expected no middlewares in router but got '%d'. %+v", len(cfg.HTTP.Routers[testComponentName].Middlewares), cfg)
	}

	if len(cfg.HTTP.Middlewares) != 0 {
		t.Errorf("Expected no middlewares but got '%d'. %+v", len(cfg.HTTP.Middlewares), cfg)
	}
}
