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

package identityprovider

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	oauth "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	exists, err := ctx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(
		ctx.Context,
		types.NamespacedName{Name: oAuthClientName},
		oauthClient,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuthClient %s: %w", oAuthClientName, err)
	} else if !exists {
		return nil, nil
	}

	return oauthClient, nil
}

func GetOAuthClientName(ctx *chetypes.DeployContext) string {
	return utils.GetValue(ctx.Authentication.ClientId, ctx.CheCluster.Namespace+"-client")
}
