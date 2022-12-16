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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testComponentName = "testComponentName"
	testRule          = "PathPrefix(`/test`)"
)

func TestCreateEmptyTraefikConfig(t *testing.T) {
	cfg := CreateEmptyTraefikConfig()

	assert.Empty(t, cfg.HTTP.Routers)
	assert.Empty(t, cfg.HTTP.Services)
	assert.Empty(t, cfg.HTTP.Middlewares)
}

func TestTraefikConfig_AddComponent(t *testing.T) {
	cfg := CreateEmptyTraefikConfig()
	cfg.AddComponent(testComponentName, testRule, 1, "http://svc", []string{})

	assert.Contains(t, cfg.HTTP.Routers, testComponentName)
	assert.Contains(t, cfg.HTTP.Services, testComponentName)
	assert.Empty(t, cfg.HTTP.Middlewares)
}

func TestStripPrefixesWhenCreating(t *testing.T) {
	check := func(cfg *TraefikConfig) {
		assert.Contains(t, cfg.HTTP.Routers[testComponentName].Middlewares, testComponentName+StripPrefixMiddlewareSuffix)
		setPrefixes := cfg.HTTP.Middlewares[testComponentName+StripPrefixMiddlewareSuffix].StripPrefix.Prefixes
		assert.Contains(t, setPrefixes, "a")
		assert.Contains(t, setPrefixes, "b")
		assert.Contains(t, setPrefixes, "c")
		assert.Len(t, setPrefixes, 3)
	}
	t.Run("addComponent", func(t *testing.T) {
		cfg := CreateEmptyTraefikConfig()
		cfg.AddComponent(testComponentName, testRule, 1, "http://svc", []string{"a", "b", "c"})

		check(cfg)
	})

	t.Run("createCommon", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc", []string{"a", "b", "c"})

		check(cfg)
	})
}

func TestAddStripPrefix(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
	cfg.AddStripPrefix(testComponentName, []string{"/test"})

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	assert.Contains(t, cfg.HTTP.Middlewares, cfg.HTTP.Routers[testComponentName].Middlewares[0], *cfg)
}

func TestAddAuthHeaderRewrite(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
	cfg.AddAuthHeaderRewrite(testComponentName)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	assert.Contains(t, cfg.HTTP.Middlewares, cfg.HTTP.Routers[testComponentName].Middlewares[0], *cfg)
}

func TestAddOpenShiftTokenCheck(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
	cfg.AddOpenShiftTokenCheck(testComponentName)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	middlewareName := cfg.HTTP.Routers[testComponentName].Middlewares[0]
	if assert.Contains(t, cfg.HTTP.Middlewares, middlewareName, *cfg) && assert.NotNil(t, cfg.HTTP.Middlewares[middlewareName].ForwardAuth) {
		assert.Equal(t, "https://kubernetes.default.svc/apis/user.openshift.io/v1/users/~", cfg.HTTP.Middlewares[middlewareName].ForwardAuth.Address)
	}
}

func TestAddErrors(t *testing.T) {
	status := "500-599"
	service := "service"
	query := "/"

	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
	cfg.AddErrors(testComponentName, status, service, query)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	middlewareName := cfg.HTTP.Routers[testComponentName].Middlewares[0]
	if assert.Contains(t, cfg.HTTP.Middlewares, middlewareName, *cfg) && assert.NotNil(t, cfg.HTTP.Middlewares[middlewareName].Errors) {
		assert.Equal(t, status, cfg.HTTP.Middlewares[middlewareName].Errors.Status)
		assert.Equal(t, service, cfg.HTTP.Middlewares[middlewareName].Errors.Service)
		assert.Equal(t, query, cfg.HTTP.Middlewares[middlewareName].Errors.Query)
	}
}

func TestAddResponseHeaders(t *testing.T) {
	reponseHeaders := map[string]string{"cache-control": "no-store, max-age=0"}

	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
	cfg.AddResponseHeaders(testComponentName, reponseHeaders)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	middlewareName := cfg.HTTP.Routers[testComponentName].Middlewares[0]
	if assert.Contains(t, cfg.HTTP.Middlewares, middlewareName, *cfg) && assert.NotNil(t, cfg.HTTP.Middlewares[middlewareName].Headers) {
		assert.Equal(t, reponseHeaders, cfg.HTTP.Middlewares[middlewareName].Headers.CustomResponseHeaders)
	}
}

func TestAddRetry(t *testing.T) {
	attempts := 3
	initialInterval := "100ms"

	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
	cfg.AddRetry(testComponentName, attempts, initialInterval)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	middlewareName := cfg.HTTP.Routers[testComponentName].Middlewares[0]
	if assert.Contains(t, cfg.HTTP.Middlewares, middlewareName, *cfg) && assert.NotNil(t, cfg.HTTP.Middlewares[middlewareName].Retry) {
		assert.Equal(t, attempts, cfg.HTTP.Middlewares[middlewareName].Retry.Attempts)
		assert.Equal(t, initialInterval, cfg.HTTP.Middlewares[middlewareName].Retry.InitialInterval)
	}
}

func TestMiddlewaresPreserveOrder(t *testing.T) {
	t.Run("strip-header", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
		cfg.AddStripPrefix(testComponentName, []string{"/test"})
		cfg.AddAuthHeaderRewrite(testComponentName)

		assert.Equal(t, testComponentName+StripPrefixMiddlewareSuffix, cfg.HTTP.Routers[testComponentName].Middlewares[0],
			"first middleware should be strip-prefix")
		assert.Equal(t, testComponentName+HeaderRewriteMiddlewareSuffix, cfg.HTTP.Routers[testComponentName].Middlewares[1],
			"second middleware should be header-rewrite")
	})

	t.Run("header-strip", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})
		cfg.AddAuthHeaderRewrite(testComponentName)
		cfg.AddStripPrefix(testComponentName, []string{"/test"})

		assert.Equal(t, testComponentName+HeaderRewriteMiddlewareSuffix, cfg.HTTP.Routers[testComponentName].Middlewares[0],
			"first middleware should be header-rewrite")
		assert.Equal(t, testComponentName+StripPrefixMiddlewareSuffix, cfg.HTTP.Routers[testComponentName].Middlewares[1],
			"second middleware should be strip-prefix")
	})
}

func TestCreateCommonTraefikConfig(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080", []string{})

	assert.Contains(t, cfg.HTTP.Routers, testComponentName)
	assert.Contains(t, cfg.HTTP.Services, testComponentName)
	assert.Empty(t, cfg.HTTP.Routers[testComponentName].Middlewares, *cfg)
	assert.Empty(t, cfg.HTTP.Middlewares, *cfg)
}
