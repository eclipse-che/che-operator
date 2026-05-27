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

package che

import (
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type oidcAuthResult struct {
	IssuerURL        string
	OIDCClientId     string
	OIDCClientSecret string
	UsernameClaim    string
	UsernamePrefix   string
	GroupsClaim      string
	GroupsPrefix     string
}

func TestResolveOIDCAuthentication(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		oAuthEnabled bool
		cheCluster   *chev2.CheCluster
		initObjects  []client.Object
		expectedAuth *oidcAuthResult
	}

	testCases := []testCase{
		{
			name:         "All fields from CheCluster spec on Kubernetes",
			isOpenShift:  false,
			oAuthEnabled: false,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityProviderURL: "https://keycloak.example.com",
							OAuthClientName:     "che-client",
							OAuthSecret:         "my-secret",
						},
					},
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_OIDC_USERNAME__CLAIM":  "preferred_username",
								"CHE_OIDC_USERNAME__PREFIX": "che:",
								"CHE_OIDC_GROUPS__CLAIM":    "groups",
								"CHE_OIDC_GROUPS__PREFIX":   "che-group:",
							},
						},
					},
				},
			},
			initObjects: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of": "eclipse-che",
						},
					},
					Data: map[string][]byte{
						"oAuthSecret": []byte("secret-value"),
					},
				},
			},
			expectedAuth: &oidcAuthResult{
				IssuerURL:        "https://keycloak.example.com",
				OIDCClientId:     "che-client",
				OIDCClientSecret: "secret-value",
				UsernameClaim:    "preferred_username",
				UsernamePrefix:   "che:",
				GroupsClaim:      "groups",
				GroupsPrefix:     "che-group:",
			},
		},
		{
			name:         "OpenShift without OAuth, claim mappings resolved from cluster Authentication",
			isOpenShift:  true,
			oAuthEnabled: false,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityProviderURL: "https://oidc.example.com",
							OAuthClientName:     "che-client",
						},
					},
				},
			},
			initObjects: []client.Object{
				&configv1.Authentication{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.AuthenticationSpec{
						Type: configv1.AuthenticationTypeOIDC,
						OIDCProviders: []configv1.OIDCProvider{
							{
								Name: "my-oidc",
								Issuer: configv1.TokenIssuer{
									URL: "https://oidc.example.com",
								},
								ClaimMappings: configv1.TokenClaimMappings{
									Username: configv1.UsernameClaimMapping{
										Claim:        "sub",
										Prefix:       &configv1.UsernamePrefix{PrefixString: "oidc-user:"},
										PrefixPolicy: configv1.Prefix,
									},
									Groups: configv1.PrefixedClaimMapping{
										TokenClaimMapping: configv1.TokenClaimMapping{
											Claim: "groups",
										},
										Prefix: "oidc:",
									},
								},
								OIDCClients: []configv1.OIDCClientConfig{
									{
										ComponentName:      "console",
										ComponentNamespace: "openshift-console",
										ClientID:           "che-client",
										ClientSecret: configv1.SecretNameReference{
											Name: "che-secret",
										},
									},
								},
							},
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "che-secret",
						Namespace: "openshift-config",
					},
					Data: map[string][]byte{
						"clientSecret": []byte("che-secret-value"),
					},
				},
			},
			expectedAuth: &oidcAuthResult{
				IssuerURL:        "https://oidc.example.com",
				OIDCClientId:     "che-client",
				OIDCClientSecret: "che-secret-value",
				UsernameClaim:    "sub",
				UsernamePrefix:   "oidc-user:",
				GroupsClaim:      "groups",
				GroupsPrefix:     "oidc:",
			},
		},
		{
			name:         "Backward compatibility: OAuthSecret treated as literal value when secret not found",
			isOpenShift:  false,
			oAuthEnabled: false,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityProviderURL: "https://keycloak.example.com",
							OAuthClientName:     "che-client",
							OAuthSecret:         "literal-secret-value",
						},
					},
				},
			},
			expectedAuth: &oidcAuthResult{
				IssuerURL:        "https://keycloak.example.com",
				OIDCClientId:     "che-client",
				OIDCClientSecret: "literal-secret-value",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.isOpenShift {
				infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)
				infrastructure.SetOpenShiftOAuthEnabledForTesting(tc.oAuthEnabled)
			} else {
				infrastructure.InitializeForTesting(infrastructure.Kubernetes)
			}

			defer func() {
				infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)
				infrastructure.SetOpenShiftOAuthEnabledForTesting(true)
			}()

			ctx := test.NewCtxBuilder().
				WithCheCluster(tc.cheCluster).
				WithObjects(tc.initObjects...).
				Build()

			auth, err := ResolveAuthentication(ctx)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			got := &oidcAuthResult{
				IssuerURL:        auth.IssuerURL,
				OIDCClientId:     auth.ClientId,
				OIDCClientSecret: string(auth.ClientSecret),
				UsernameClaim:    auth.UsernameClaim,
				UsernamePrefix:   auth.UsernamePrefix,
				GroupsClaim:      auth.GroupsClaim,
				GroupsPrefix:     auth.GroupsPrefix,
			}

			if diff := cmp.Diff(tc.expectedAuth, got); diff != "" {
				t.Errorf("Authentication result mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
