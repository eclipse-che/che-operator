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

	openShiftNamespaceOIDCClientSecretKey = "clientSecret"
	cheNamespaceOIDCClientSecretKey       = "oAuthSecret"
)

// ResolveOIDCAuthentication builds an Authentication config from the CheCluster spec,
// falling back to the OpenShift cluster Authentication resource for any unset fields.
func ResolveOIDCAuthentication(ctx *chetypes.DeployContext) (*chetypes.OIDCAuthentication, error) {
	authentication := &chetypes.OIDCAuthentication{
		UsernameClaim:  ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_USERNAME__CLAIM"],
		UsernamePrefix: ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_USERNAME__PREFIX"],
		GroupsClaim:    ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_GROUPS__CLAIM"],
		GroupsPrefix:   ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_GROUPS__PREFIX"],
		IssuerURL:      ctx.CheCluster.Spec.Networking.Auth.IdentityProviderURL,
		OIDCClientId:   ctx.CheCluster.Spec.Networking.Auth.OAuthClientName,
	}

	if ctx.CheCluster.Spec.Networking.Auth.OAuthSecret != "" {
		oidcClientSecret, err := resolveOIDCClientSecret(ctx.CheCluster.Spec.Networking.Auth.OAuthSecret, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve OIDC client secret: %w", err)
		}
		authentication.OIDCClientSecret = oidcClientSecret
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

		if authentication.IssuerURL == "" {
			authentication.IssuerURL = oidcProvider.Issuer.URL

			// Sync issuer CA
			if oidcProvider.Issuer.CertificateAuthority.Name != "" {
				issuerCA, err := readIssuerCA(
					oidcProvider.Issuer.CertificateAuthority.Name,
					openshiftConfigNamespace,
					ctx,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to sync issuer CA: %w", err)
				}
				authentication.IssuerCA = issuerCA
			}
		}

		if authentication.UsernameClaim == "" {
			authentication.UsernameClaim = oidcProvider.ClaimMappings.Username.Claim
		}

		if authentication.UsernamePrefix == "" {
			if authentication.UsernamePrefix == "" {
				switch oidcProvider.ClaimMappings.Username.PrefixPolicy {
				case configv1.NoOpinion:
					if authentication.UsernameClaim != "email" {
						authentication.UsernamePrefix = fmt.Sprintf("%s#", authentication.IssuerURL)
					}
				case configv1.Prefix:
					if oidcProvider.ClaimMappings.Username.Prefix != nil {
						authentication.UsernamePrefix = oidcProvider.ClaimMappings.Username.Prefix.PrefixString
					}
				}
			}
		}

		if authentication.GroupsClaim == "" {
			authentication.GroupsClaim = oidcProvider.ClaimMappings.Groups.Claim
		}

		if authentication.GroupsPrefix == "" {
			authentication.GroupsPrefix = oidcProvider.ClaimMappings.Groups.Prefix
		}

		if authentication.OIDCClientId == "" {
			// Reuse the console's OIDC client credentials when no explicit client is configured.
			for _, oidcClient := range oidcProvider.OIDCClients {
				if oidcClient.ComponentName == "console" {
					authentication.OIDCClientId = oidcClient.ClientID

					if oidcClient.ClientSecret.Name != "" {
						oidcClientSecret, err := readOIDCClientSecretFromOpenShiftNamespace(oidcClient.ClientSecret.Name, ctx)
						if err != nil {
							return nil, fmt.Errorf("failed to read OIDC client secret: %w", err)
						}
						authentication.OIDCClientSecret = oidcClientSecret
					}

					break
				}
			}

			if authentication.OIDCClientId == "" {
				return nil, fmt.Errorf("failed to find `openshift-console` OIDC client")
			}
		}
	}

	return authentication, nil
}

func readOIDCClientSecretFromOpenShiftNamespace(oidcClientSecretName string, ctx *chetypes.DeployContext) ([]byte, error) {
	secret := &corev1.Secret{}

	// Use client instead of wrapper in order to catch all errors including NotFound
	err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      oidcClientSecretName,
			Namespace: openshiftConfigNamespace,
		},
		secret,
	)
	if err != nil {
		return nil, err
	}

	value, ok := secret.Data[openShiftNamespaceOIDCClientSecretKey]
	if !ok {
		return nil, fmt.Errorf("no client secret found")
	}

	return value, nil
}

func resolveOIDCClientSecret(oidcClientSecret string, ctx *chetypes.DeployContext) ([]byte, error) {
	secret := &corev1.Secret{}

	// treat as Secret name first
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      oidcClientSecret,
			Namespace: ctx.CheCluster.Namespace,
		},
		secret,
	)
	if err != nil {
		return nil, err
	} else if exists {
		value, ok := secret.Data[cheNamespaceOIDCClientSecretKey]
		if !ok {
			return nil, fmt.Errorf("failed to fetch OIDC client secret: no client secret found")
		}

		return value, nil
	}

	// Backward compatibility: treat OIDCClientSecretName as a literal secret value, not a reference.
	return []byte(oidcClientSecret), nil
}

func readIssuerCA(
	issuerCAConfigMapName string,
	issuerCAConfigMapNamespace string,
	ctx *chetypes.DeployContext,
) (string, error) {
	cm := &corev1.ConfigMap{}

	err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      issuerCAConfigMapName,
			Namespace: issuerCAConfigMapNamespace,
		},
		cm,
	)
	if err != nil {
		return "", err
	}

	return cm.Data[issuerCAKey], nil
}
