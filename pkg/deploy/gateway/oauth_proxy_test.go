//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

	"k8s.io/utils/pointer"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCookieExpireForOpenShiftOauthProxyConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						OAuthProxy: &chev2.OAuthProxy{
							CookieExpireSeconds: pointer.Int32(3665),
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
							CookieExpireSeconds: pointer.Int32(3665),
						},
					},
				},
			}},
	}).Build()

	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "")
	assert.Contains(t, config, "cookie_expire = \"1h1m5s\"")
}

func TestKubernetesOauthProxySecretSecretFoundWithKey(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthSecret: "my-secret",
				},
			}},
	}).WithObjects(&corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "eclipse-che",
			Labels:    map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"oAuthSecret": []byte("my")},
	},
	).Build()

	ctx.CheHost = "che-site.che-domain.com"
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	assert.Contains(t, config, "client_secret = \"my\"")
}

func TestKubernetesOauthProxySecretSecretFoundWithWrongKey(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthSecret: "my-secret",
				},
			}},
	}).WithObjects(&corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "eclipse-che",
			Labels:    map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"keyIsNotoAuthSecret": []byte("my")},
	}).Build()

	ctx.CheHost = "che-site.che-domain.com"
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	//expect interpret as literal secret
	assert.Contains(t, config, "client_secret = \"my-secret\"")
}

func TestKubernetesOauthProxySecretSecretFoundWithWrongSecretName(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthSecret: "wrong-secret-name",
				},
			}},
	}).WithObjects(&corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "eclipse-che",
			Labels:    map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"oAuthSecret": []byte("my")},
	}).Build()
	ctx.CheHost = "che-site.che-domain.com"
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	//expect interpret as literal secret
	assert.Contains(t, config, "client_secret = \"wrong-secret-name\"")
}

func TestKubernetesOauthProxySecretLegacyPlaintextSecretName(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthSecret: "abcdefPlainTextSecret",
				},
			},
		},
	}).Build()
	ctx.CheHost = "che-site.che-domain.com"
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	config := kubernetesOauthProxyConfig(ctx, "blabol")
	//expect interpret as literal secret
	assert.Contains(t, config, "client_secret = \"abcdefPlainTextSecret\"")
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
