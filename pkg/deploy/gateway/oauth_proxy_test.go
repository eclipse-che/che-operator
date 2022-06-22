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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
)

func TestKubernetesOauthProxyConfig(t *testing.T) {
	ctx := test.GetDeployContext(
		&chev2.CheCluster{
			Spec: chev2.CheClusterSpec{
				Networking: chev2.CheClusterSpecNetworking{
					Auth: chev2.Auth{
						IdentityProviderURL: "http://bla.bla.bla/idp",
						OAuthClientName:     "client name",
						OAuthSecret:         "secret",
					},
				}},
		}, nil)
	ctx.CheHost = "che-site.che-domain.com"
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	assert.Contains(t, config, "pass_authorization_header = true")
	assert.Contains(t, config, "whitelist_domains = \".che-domain.com\"")
	assert.Contains(t, config, "cookie_domains = \".che-domain.com\"")
	assert.NotContains(t, config, "scope = ")
	assert.NotContains(t, config, "pass_access_token = true")
}

func TestScopeDefinedForKubernetesOauthProxyConfig(t *testing.T) {
	ctx := test.GetDeployContext(
		&chev2.CheCluster{
			Spec: chev2.CheClusterSpec{
				Networking: chev2.CheClusterSpecNetworking{
					Auth: chev2.Auth{
						IdentityProviderURL: "http://bla.bla.bla/idp",
						OAuthClientName:     "client name",
						OAuthSecret:         "secret",
						OAuthScope:          "scope1 scope2 scope3 scope4 scope5",
					},
				}},
		}, nil)
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	assert.Contains(t, config, "scope = \"scope1 scope2 scope3 scope4 scope5\"")
}

func TestAccessTokenDefinedForKubernetesOauthProxyConfig(t *testing.T) {
	ctx := test.GetDeployContext(
		&chev2.CheCluster{
			Spec: chev2.CheClusterSpec{
				Networking: chev2.CheClusterSpecNetworking{
					Auth: chev2.Auth{
						IdentityProviderURL: "http://bla.bla.bla/idp",
						OAuthClientName:     "client name",
						OAuthSecret:         "secret",
						IdentityToken:       "access_token",
					},
				}},
		}, nil)
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	assert.Contains(t, config, "pass_access_token = true")
	assert.NotContains(t, config, "pass_authorization_header = true")
}
