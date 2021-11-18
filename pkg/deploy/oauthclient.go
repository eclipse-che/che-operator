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
package deploy

import (
	"strings"

	oauth "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	OAuthFinalizerName = "oauthclients.finalizers.che.eclipse.org"
)

func GetKeycloakOAuthClientSpec(name string, oauthSecret string, keycloakURL string, keycloakRealm string, isOpenShift4 bool) *oauth.OAuthClient {
	providerName := "openshift-v3"
	if isOpenShift4 {
		providerName = "openshift-v4"
	}

	redirectURLSuffix := "/realms/" + keycloakRealm + "/broker/" + providerName + "/endpoint"
	redirectURIs := []string{
		keycloakURL + redirectURLSuffix,
	}

	keycloakURL = strings.NewReplacer("https://", "", "http://", "").Replace(keycloakURL)
	if !strings.Contains(keycloakURL, "://") {
		redirectURIs = []string{
			"http://" + keycloakURL + redirectURLSuffix,
			"https://" + keycloakURL + redirectURLSuffix,
		}
	}
	return GetOAuthClientSpec(name, oauthSecret, redirectURIs)
}

func GetOAuthClientSpec(name string, oauthSecret string, redirectURIs []string) *oauth.OAuthClient {
	return &oauth.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: oauth.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": "che"},
		},

		Secret:       oauthSecret,
		RedirectURIs: redirectURIs,
		GrantMethod:  oauth.GrantHandlerPrompt,
	}
}

func ReconcileOAuthClientFinalizer(deployContext *DeployContext) (err error) {
	cheCluster := deployContext.CheCluster
	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return AppendFinalizer(deployContext, OAuthFinalizerName)
	} else {
		oAuthClientName := cheCluster.Spec.Auth.OAuthClientName
		return DeleteObjectWithFinalizer(deployContext, types.NamespacedName{Name: oAuthClientName}, &oauth.OAuthClient{}, OAuthFinalizerName)
	}
}
