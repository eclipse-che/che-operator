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
package identityprovider

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	oauth "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetOAuthClientSpec(
	name string,
	secret string,
	redirectURIs []string,
	accessTokenInactivityTimeoutSeconds *int32,
	accessTokenMaxAgeSeconds *int32) *oauth.OAuthClient {

	return &oauth.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: oauth.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
		},

		Secret:                              secret,
		RedirectURIs:                        redirectURIs,
		GrantMethod:                         oauth.GrantHandlerPrompt,
		AccessTokenInactivityTimeoutSeconds: accessTokenInactivityTimeoutSeconds,
		AccessTokenMaxAgeSeconds:            accessTokenMaxAgeSeconds,
	}
}

func GetOAuthClient(ctx *chetypes.DeployContext) (*oauth.OAuthClient, error) {
	oAuthClientName := GetOAuthClientName(ctx)

	oauthClient := &oauth.OAuthClient{}
	exists, err := deploy.GetClusterObject(ctx, oAuthClientName, oauthClient)
	if !exists {
		return nil, err
	}

	return oauthClient, nil
}

func GetOAuthClientName(ctx *chetypes.DeployContext) string {
	if ctx.CheCluster.Spec.Networking.Auth.OAuthClientName != "" {
		return ctx.CheCluster.Spec.Networking.Auth.OAuthClientName
	}

	return ctx.CheCluster.Namespace + "-client"
}
