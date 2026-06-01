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
	"context"
	"fmt"
	"slices"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// OpenShift external OIDC authentication constants.
	// See: https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/authentication_and_authorization/external-auth
	openshiftConfigNamespace = "openshift-config"
	issuerCAKey              = "ca-bundle.crt"
	oidcClientSecretKey      = "clientSecret"
)

// ResolveAuthentication builds an Authentication config from the CheCluster spec,
// falling back to the OpenShift cluster Authentication resource for any unset fields.
func ResolveAuthentication(ctx *chetypes.DeployContext) (*chetypes.Authentication, error) {
	authentication := &chetypes.Authentication{
		UsernameClaim:  ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_USERNAME__CLAIM"],
		UsernamePrefix: ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_USERNAME__PREFIX"],
		GroupsClaim:    ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_GROUPS__CLAIM"],
		GroupsPrefix:   ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_GROUPS__PREFIX"],
		IssuerURL:      ctx.CheCluster.Spec.Networking.Auth.IdentityProviderURL,
		// OIDC client ID must be explicitly defined; the openshift-console client
		// cannot be reused because it has a different callback URL.
		ClientId: ctx.CheCluster.Spec.Networking.Auth.OAuthClientName,
	}

	// must be outside main `if` condition
	if ctx.CheCluster.Spec.Networking.Auth.OAuthSecret != "" {
		// `OAuthSecret` can be a Kubernetes Secret name in CheCluster namespace
		// or a literal value; resolve accordingly.
		clientSecret, err := resolveOAuthSecretInCheNamespace(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve secret: %w", err)
		}
		authentication.ClientSecret = clientSecret
	}

	if infrastructure.IsOpenShiftExternalAuth() {
		clusterAuthentication := &configv1.Authentication{}
		err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, clusterAuthentication)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch authentication config: %w", err)
		}

		if clusterAuthentication.Spec.Type != configv1.AuthenticationTypeOIDC {
			return nil, fmt.Errorf("authentication type is not OIDC")
		}

		if len(clusterAuthentication.Spec.OIDCProviders) == 0 {
			return nil, fmt.Errorf("no OIDC providers configured")
		}

		if len(clusterAuthentication.Spec.OIDCProviders) != 1 {
			return nil, fmt.Errorf("multiple OIDC providers configured, expected exactly one")
		}

		oidcProvider := clusterAuthentication.Spec.OIDCProviders[0]

		// issuer URL
		if authentication.IssuerURL == "" {
			authentication.IssuerURL = oidcProvider.Issuer.URL
		}

		// issuer certificate authority
		if authentication.IssuerURL == oidcProvider.Issuer.URL {
			if oidcProvider.Issuer.CertificateAuthority.Name != "" {
				issuerCA, err := readIssuerCA(oidcProvider.Issuer.CertificateAuthority.Name, ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to read issuer CA: %w", err)
				}
				authentication.IssuerCA = issuerCA
			}
		}

		// username/groups claim mappings
		if authentication.GroupsClaim == "" {
			authentication.GroupsClaim = oidcProvider.ClaimMappings.Groups.Claim
		}
		if authentication.GroupsClaim != "" && authentication.GroupsPrefix == "" {
			authentication.GroupsPrefix = oidcProvider.ClaimMappings.Groups.Prefix
		}

		if authentication.UsernameClaim == "" {
			authentication.UsernameClaim = oidcProvider.ClaimMappings.Username.Claim
		}
		if authentication.UsernameClaim != "" && authentication.UsernamePrefix == "" {
			switch oidcProvider.ClaimMappings.Username.PrefixPolicy {
			case configv1.NoOpinion:
				// See `NoOpinion` description
				if authentication.UsernameClaim != "email" {
					authentication.UsernamePrefix = fmt.Sprintf("%s#", authentication.IssuerURL)
				}
			case configv1.Prefix:
				if oidcProvider.ClaimMappings.Username.Prefix != nil {
					authentication.UsernamePrefix = oidcProvider.ClaimMappings.Username.Prefix.PrefixString
				}
			}
		}

		// client secret
		if len(authentication.ClientSecret) == 0 {
			idx := slices.IndexFunc(oidcProvider.OIDCClients, func(config configv1.OIDCClientConfig) bool {
				return config.ClientID == authentication.ClientId
			})

			if idx != -1 {
				oidcClient := oidcProvider.OIDCClients[idx]
				if oidcClient.ClientSecret.Name != "" {
					clientSecret, err := resolveClientSecretInOpenShiftConfigNamespace(oidcClient.ClientSecret.Name, ctx)
					if err != nil {
						return nil, fmt.Errorf("failed to read client secret: %w", err)
					}
					authentication.ClientSecret = clientSecret
				}
			}
		}
	}

	return authentication, nil
}

func resolveClientSecretInOpenShiftConfigNamespace(secretName string, ctx *chetypes.DeployContext) ([]byte, error) {
	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{Name: secretName, Namespace: openshiftConfigNamespace},
		secret,
	)
	if err != nil {
		return nil, err
	}

	value, ok := secret.Data[oidcClientSecretKey]
	if ok {
		return value, nil
	}

	return nil, fmt.Errorf("client secret not found in: %s", secretName)
}

func resolveOAuthSecretInCheNamespace(ctx *chetypes.DeployContext) ([]byte, error) {
	secret := &corev1.Secret{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      ctx.CheCluster.Spec.Networking.Auth.OAuthSecret,
			Namespace: ctx.CheCluster.Namespace,
		},
		secret,
	)
	if err != nil {
		return nil, err
	}
	if exists {
		value, ok := secret.Data["oAuthSecret"]
		if ok {
			return value, nil
		}

		return nil, fmt.Errorf("client secret not found in: %s", ctx.CheCluster.Spec.Networking.Auth.OAuthSecret)
	}

	// Backward compatibility: treat as a literal secret value, not a reference.
	return []byte(ctx.CheCluster.Spec.Networking.Auth.OAuthSecret), nil
}

func readIssuerCA(cmName string, ctx *chetypes.DeployContext) (string, error) {
	cm := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{Name: cmName, Namespace: openshiftConfigNamespace},
		cm,
	)
	if err != nil {
		return "", err
	}

	ca, ok := cm.Data[issuerCAKey]
	if !ok {
		return "", fmt.Errorf("issuer CA not found in the ConfigMap %s", cmName)
	}

	return ca, nil
}
