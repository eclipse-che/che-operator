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

package checontext

import (
	"context"
	"strings"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/cheproxy"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Builder struct {
	cheCluster *chev2.CheCluster
	initObject []client.Object
}

func NewBuilder() *Builder {
	return &Builder{
		cheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "CheCluster",
				APIVersion: chev2.GroupVersion.String(),
			},
		},
	}
}

func (b *Builder) WithObjects(initObjs ...client.Object) *Builder {
	b.initObject = append(b.initObject, initObjs...)
	return b
}

func (b *Builder) WithCheCluster(cheCluster *chev2.CheCluster) *Builder {
	b.cheCluster = cheCluster

	if cheCluster != nil {
		b.cheCluster.TypeMeta = metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		}
	}

	return b
}

func (b *Builder) WithEmptyCheCluster() *Builder {
	b.cheCluster = &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
	}
	return b
}

func (b *Builder) Build() *chetypes.CheContext {
	if b.cheCluster != nil {
		if b.cheCluster.Spec.Networking.Domain != "" {
			b.cheCluster.Status.WorkspaceBaseDomain = b.cheCluster.Spec.Networking.Domain
		}
		if b.cheCluster.Spec.Networking.Hostname != "" {
			b.cheCluster.Status.CheURL = "https://" + b.cheCluster.Spec.Networking.Hostname
		}
		if b.cheCluster.Status.CheURL == "" {
			b.cheCluster.Status.CheURL = "https://che-host"
		}
		b.initObject = append(b.initObject, b.cheCluster)
	}

	for _, obj := range b.initObject {
		if obj.GetNamespace() != "" {
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: obj.GetNamespace()}}
			err := client.IgnoreAlreadyExists(clusterapi.Get().Client.Create(context.Background(), namespace))
			if err != nil {
				panic("failed to build CheContext: " + err.Error())
			}
		}

		err := clusterapi.Get().Client.Create(context.Background(), obj)
		if err != nil {
			panic("failed to build CheContext: " + err.Error())
		}
	}

	cheCtx := &chetypes.CheContext{
		CheCluster:     b.cheCluster,
		ClusterAPI:     clusterapi.Get(),
		Context:        context.Background(),
		Proxy:          &cheproxy.Proxy{},
		Authentication: buildCheAuthentication(b.cheCluster),
		DWONamespace:   "devworkspace-controller",
	}

	if b.cheCluster != nil {
		cheCtx.CheHost = strings.TrimPrefix(b.cheCluster.Status.CheURL, "https://")
	}

	return cheCtx
}

func buildCheAuthentication(cheCluster *chev2.CheCluster) *chetypes.Authentication {
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
