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
package v2

import (
	"reflect"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestIsAccesTokenConfigured(t *testing.T) {
	t.Run("TestIsAccesTokenConfigured when access_token defined", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "access_token",
					},
				}},
		}
		assert.True(t, cheCluster.IsAccessTokenConfigured(), "'access_token' should be activated")
	})
	t.Run("TestIsAccesTokenConfigured when id_token defined", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "id_token",
					},
				}},
		}
		assert.False(t, cheCluster.IsAccessTokenConfigured(), "'access_token' should not be activated")
	})
}

func TestGetIdentityToken(t *testing.T) {
	t.Run("TestGetIdentityToken when access_token defined in config and k8s", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "access_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, "access_token", cheCluster.GetIdentityToken(),
			"'access_token' should be used")
	})

	t.Run("TestGetIdentityToken when id_token defined in config and k8s", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "id_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, "id_token", cheCluster.GetIdentityToken(),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when no defined token in config and k8s", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, "id_token", cheCluster.GetIdentityToken(),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when access_token defined in config and openshift", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "access_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, "access_token", cheCluster.GetIdentityToken(),
			"'access_token' should be used")
	})

	t.Run("TestGetIdentityToken when id_token defined in config and openshift", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "id_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, "id_token", cheCluster.GetIdentityToken(),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when no defined token in config and openshift", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, "access_token", cheCluster.GetIdentityToken(),
			"'access_token' should be used")
	})

}

func TestGetDefaultIdentityToken(t *testing.T) {
	emptyCheCluster := CheCluster{}

	var tests = []struct {
		infrastructure infrastructure.Type
		identityToken  string
	}{
		{infrastructure.OpenShiftv4, "access_token"},
		{infrastructure.Kubernetes, "id_token"},
		{infrastructure.Unsupported, "id_token"},
	}
	for _, test := range tests {
		infrastructure.InitializeForTesting(test.infrastructure)
		if actual := emptyCheCluster.GetIdentityToken(); !reflect.DeepEqual(test.identityToken, actual) {
			t.Errorf("Test Failed. Expected '%s', but got '%s'", test.identityToken, actual)
		}
	}
}

func TestGetOAuthProxyEnvVars(t *testing.T) {
	cheCluster := CheCluster{
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{
				Auth: Auth{
					OAuthProxyExtraProperties: map[string]string{
						"OAUTH2_PROXY_COOKIE_SECRET":          "ssdjflsd",
						"CHE_DEVWORKSPACES_ENABLED":           "false",
						"OAUTH2_PROXY_EMAIL_DOMAINS":          "*",
						"KUBERNETES_LABELS":                   "app.kubernetes.io/component=che",
						"OAUTH2_PROXY_SKIP_JWT_BEARER_TOKENS": "true",
						"JAVA_OPTS":                           "-XX:MaxRAMPercentage=85.0",
						"OAUTH2_PROXY_EXTRA_JWT_ISSUERS":      "[\"https://abc.zcx.net/84jd4m2wd/=29sdjr38-efj23\"]",
						"HTTP2_DISABLE":                       "true",
					},
				},
			}},
	}

	envVars := cheCluster.GetOAuthProxyEnvVars()

	assert.Contains(t, envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_COOKIE_SECRET", Value: "ssdjflsd"})
	assert.NotContains(t, envVars, corev1.EnvVar{Name: "CHE_DEVWORKSPACES_ENABLED", Value: "false"})
	assert.Contains(t, envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_EMAIL_DOMAINS", Value: "*"})
	assert.NotContains(t, envVars, corev1.EnvVar{Name: "KUBERNETES_LABELS", Value: "app.kubernetes.io/component=che"})
	assert.Contains(t, envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_SKIP_JWT_BEARER_TOKENS", Value: "true"})
	assert.NotContains(t, envVars, corev1.EnvVar{Name: "JAVA_OPTS", Value: "-XX:MaxRAMPercentage=85.0"})
	assert.Contains(t, envVars, corev1.EnvVar{Name: "OAUTH2_PROXY_EXTRA_JWT_ISSUERS", Value: "[\"https://abc.zcx.net/84jd4m2wd/=29sdjr38-efj23\"]"})
	assert.NotContains(t, envVars, corev1.EnvVar{Name: "HTTP2_DISABLE", Value: "true"})
}
