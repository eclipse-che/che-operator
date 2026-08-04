//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

	"k8s.io/utils/ptr"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCookieExpireForOpenShiftOauthProxyConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						OAuthProxy: &chev2.OAuthProxy{
							CookieExpireSeconds: ptr.To(int32(3665)),
						},
					},
				},
			}},
	}).Build()

	config := openshiftOauthProxyConfig(ctx, "")
	assert.Contains(t, config, "cookie_expire = \"1h1m5s\"")
}

func TestCookieExpireKubernetesOauthProxyConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						OAuthProxy: &chev2.OAuthProxy{
							CookieExpireSeconds: ptr.To(int32(3665)),
						},
					},
				},
			}},
	}).Build()

	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "")
	assert.Contains(t, config, "cookie_expire = \"1h1m5s\"")
}

func TestKubernetesOauthProxyConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			Spec: chev2.CheClusterSpec{
				Networking: chev2.CheClusterSpecNetworking{
					Auth: chev2.Auth{
						IdentityProviderURL: "http://bla.bla.bla/idp",
						OAuthClientName:     "client name",
						OAuthSecret:         "secret",
					},
				}},
		}).Build()
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
	ctx := test.NewCtxBuilder().WithCheCluster(
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
		}).Build()
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	assert.Contains(t, config, "scope = \"scope1 scope2 scope3 scope4 scope5\"")
}

func TestAccessTokenDefinedForKubernetesOauthProxyConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
		}).Build()
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	assert.Contains(t, config, "pass_access_token = true")
	assert.NotContains(t, config, "pass_authorization_header = true")
}

// TestResolveOpenShiftOAuthProxyImage_ImageStreamPresent verifies that when the
// openshift/oauth-proxy ImageStream is present the architecture-native digest-pinned
// image from the cluster's release payload is returned.
func TestResolveOpenShiftOAuthProxyImage_ImageStreamPresent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	expectedImage := "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:503de130e594b7864ab9b63b910d313b4e17cdd09ddd59323729fa221ba1b39c"

	imageStream := &unstructured.Unstructured{}
	imageStream.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Version: "v1",
		Kind:    "ImageStream",
	})
	imageStream.SetName("oauth-proxy")
	imageStream.SetNamespace("openshift")
	_ = unstructured.SetNestedSlice(imageStream.Object, []interface{}{
		map[string]interface{}{
			"tag": "v4.4",
			"items": []interface{}{
				map[string]interface{}{
					"dockerImageReference": expectedImage,
				},
			},
		},
	}, "status", "tags")

	ctx := test.NewCtxBuilder().WithObjects(imageStream).Build()

	resolved := resolveOpenShiftOAuthProxyImage(ctx)
	assert.Equal(t, expectedImage, resolved)
}

// TestResolveOpenShiftOAuthProxyImage_ImageStreamAbsent verifies that an empty string
// is returned when the ImageStream is not present so the caller falls back to the default.
func TestResolveOpenShiftOAuthProxyImage_ImageStreamAbsent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	ctx := test.NewCtxBuilder().Build()

	resolved := resolveOpenShiftOAuthProxyImage(ctx)
	assert.Equal(t, "", resolved)
}
