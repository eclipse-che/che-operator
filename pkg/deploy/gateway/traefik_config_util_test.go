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

func TestAddStripPrefix(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
	cfg.AddStripPrefix(testComponentName, []string{"/test"})

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	assert.Contains(t, cfg.HTTP.Middlewares, cfg.HTTP.Routers[testComponentName].Middlewares[0], *cfg)
}

func TestAddAuthHeaderRewrite(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
	cfg.AddAuthHeaderRewrite(testComponentName)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	assert.Contains(t, cfg.HTTP.Middlewares, cfg.HTTP.Routers[testComponentName].Middlewares[0], *cfg)
}

func TestAddOpenShiftTokenCheck(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
	cfg.AddOpenShiftTokenCheck(testComponentName)

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 1, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 1, *cfg)
	middlewareName := cfg.HTTP.Routers[testComponentName].Middlewares[0]
	if assert.Contains(t, cfg.HTTP.Middlewares, middlewareName, *cfg) && assert.NotNil(t, cfg.HTTP.Middlewares[middlewareName].ForwardAuth) {
		assert.Equal(t, "https://kubernetes.default.svc/apis/user.openshift.io/v1/users/~", cfg.HTTP.Middlewares[middlewareName].ForwardAuth.Address)
	}
}

func TestMiddlewaresPreserveOrder(t *testing.T) {
	t.Run("strip-header", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
		cfg.AddStripPrefix(testComponentName, []string{"/test"})
		cfg.AddAuthHeaderRewrite(testComponentName)

		assert.Equal(t, testComponentName+"-strip-prefix", cfg.HTTP.Routers[testComponentName].Middlewares[0],
			"first middleware should be strip-prefix")
		assert.Equal(t, testComponentName+"-header-rewrite", cfg.HTTP.Routers[testComponentName].Middlewares[1],
			"second middleware should be header-rewrite")
	})

	t.Run("header-strip", func(t *testing.T) {
		cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")
		cfg.AddAuthHeaderRewrite(testComponentName)
		cfg.AddStripPrefix(testComponentName, []string{"/test"})

		assert.Equal(t, testComponentName+"-header-rewrite", cfg.HTTP.Routers[testComponentName].Middlewares[0],
			"first middleware should be header-rewrite")
		assert.Equal(t, testComponentName+"-strip-prefix", cfg.HTTP.Routers[testComponentName].Middlewares[1],
			"second middleware should be strip-prefix")
	})
}

func TestCreateCommonTraefikConfig(t *testing.T) {
	cfg := CreateCommonTraefikConfig(testComponentName, testRule, 1, "http://svc:8080")

	assert.Len(t, cfg.HTTP.Routers[testComponentName].Middlewares, 0, *cfg)
	assert.Len(t, cfg.HTTP.Middlewares, 0, *cfg)
}
