//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	oauth "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


func NewOAuthClient(name string, oauthSecret string, keycloakURL string, keycloakRealm string) *oauth.OAuthClient {
	return &oauth.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: oauth.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Labels: map[string]string{"app":"che"},
		},

		Secret: oauthSecret,
		RedirectURIs: []string{
			keycloakURL + "/auth/realms/" + keycloakRealm +"/broker/openshift-v3/endpoint",
		},
		GrantMethod: oauth.GrantHandlerPrompt,
	}

}
