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
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	oauth "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},

		Secret:                              secret,
		RedirectURIs:                        redirectURIs,
		GrantMethod:                         oauth.GrantHandlerPrompt,
		AccessTokenInactivityTimeoutSeconds: accessTokenInactivityTimeoutSeconds,
		AccessTokenMaxAgeSeconds:            accessTokenMaxAgeSeconds,
	}
}

func FindOAuthClient(ctx *chetypes.DeployContext) (*oauth.OAuthClient, error) {
	if ctx.CheCluster.Spec.Networking.Auth.OAuthClientName != "" {
		oauthClient := &oauth.OAuthClient{}
		if err := ctx.ClusterAPI.Client.Get(
			context.TODO(),
			types.NamespacedName{Name: ctx.CheCluster.Spec.Networking.Auth.OAuthClientName},
			oauthClient); err != nil {
			return nil, err
		}
		return oauthClient, nil
	}

	oauthClients, err := FindAllEclipseCheOAuthClients(ctx)
	if err != nil {
		return nil, err
	}

	switch len(oauthClients) {
	case 0:
		return nil, nil
	case 1:
		return &oauthClients[0], nil
	default:
		return nil, fmt.Errorf("more than one OAuthClient found with '%s:%s' labels", constants.KubernetesPartOfLabelKey, constants.CheEclipseOrg)
	}
}

func FindAllEclipseCheOAuthClients(ctx *chetypes.DeployContext) ([]oauth.OAuthClient, error) {
	oauthClients := &oauth.OAuthClientList{}
	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(deploy.GetLabels(defaults.GetCheFlavor()))}

	if err := ctx.ClusterAPI.Client.List(
		context.TODO(),
		oauthClients,
		listOptions); err != nil {
		return nil, err
	}
	return oauthClients.Items, nil
}
