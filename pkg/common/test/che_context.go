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

package test

import (
	"context"
	"strings"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8s_client "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	testclient "github.com/eclipse-che/che-operator/pkg/common/test/test-client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheContextBuild struct {
	cheCluster *chev2.CheCluster
	initObject []client.Object
}

func NewCtxBuilder() *CheContextBuild {
	return &CheContextBuild{
		initObject: []client.Object{},
		cheCluster: getDefaultCheCluster(),
	}
}

func (f *CheContextBuild) WithObjects(initObjs ...client.Object) *CheContextBuild {
	f.initObject = append(f.initObject, initObjs...)
	return f
}

func (f *CheContextBuild) WithCheCluster(cheCluster *chev2.CheCluster) *CheContextBuild {
	f.cheCluster = cheCluster
	if f.cheCluster != nil {
		f.cheCluster.TypeMeta = metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		}
		if f.cheCluster.Status.WorkspaceBaseDomain == "" {
			f.cheCluster.Status.WorkspaceBaseDomain = f.cheCluster.Spec.Networking.Domain
		}
		if f.cheCluster.Status.CheURL == "" {
			f.cheCluster.Status.CheURL = "https://" + f.cheCluster.Spec.Networking.Hostname
		}
	}
	return f
}

func (f *CheContextBuild) Build() *chetypes.CheContext {
	if f.cheCluster != nil {
		f.initObject = append(f.initObject, f.cheCluster)
	}

	fakeClient, discoveryClient, scheme := testclient.GetTestClients(f.initObject...)

	cheCtx := &chetypes.CheContext{
		CheCluster: f.cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:                  fakeClient,
			NonCachingClient:        fakeClient,
			Scheme:                  scheme,
			DiscoveryClient:         discoveryClient,
			ClientWrapper:           k8s_client.NewK8sClient(fakeClient, scheme),
			NonCachingClientWrapper: k8s_client.NewK8sClient(fakeClient, scheme),
		},
		Proxy:          &chetypes.Proxy{},
		Authentication: buildAuthentication(f.cheCluster),
		DWONamespace:   "devworkspace-controller",
		Context:        context.Background(),
	}

	if f.cheCluster != nil {
		cheCtx.CheHost = strings.TrimPrefix(f.cheCluster.Status.CheURL, "https://")
	}

	return cheCtx
}

func buildAuthentication(cheCluster *chev2.CheCluster) *chetypes.Authentication {
	if cheCluster == nil {
		return &chetypes.Authentication{}
	}
	return &chetypes.Authentication{
		IssuerURL:      cheCluster.Spec.Networking.Auth.IdentityProviderURL,
		ClientId:       cheCluster.Spec.Networking.Auth.OAuthClientName,
		ClientSecret:   []byte(cheCluster.Spec.Networking.Auth.OAuthSecret),
		UsernameClaim:  cheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_USERNAME__CLAIM"],
		UsernamePrefix: cheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_USERNAME__PREFIX"],
		GroupsClaim:    cheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_GROUPS__CLAIM"],
		GroupsPrefix:   cheCluster.Spec.Components.CheServer.ExtraProperties["CHE_OIDC_GROUPS__PREFIX"],
	}
}

func getDefaultCheCluster() *chev2.CheCluster {
	return &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}
}
