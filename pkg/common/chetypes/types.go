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

package chetypes

import (
	"context"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheContext struct {
	CheCluster              *chev2.CheCluster
	ClusterAPI              ClusterAPI
	Proxy                   *Proxy
	Authentication          *Authentication
	IsSelfSignedCertificate bool
	CheHost                 string
	DWONamespace            string
	Context                 context.Context
}

type ClusterAPI struct {
	Client                  client.Client // We should ClientWrapper instead wherever it possible
	NonCachingClient        client.Client // We should NonCachingClientWrapper instead wherever it possible
	DiscoveryClient         discovery.DiscoveryInterface
	Scheme                  *runtime.Scheme
	ClientWrapper           clientwrapper.ClientWrapper
	NonCachingClientWrapper clientwrapper.ClientWrapper
}

type Authentication struct {
	GroupsClaim    string
	GroupsPrefix   string
	UsernameClaim  string
	UsernamePrefix string

	IssuerURL string
	IssuerCA  string

	ClientId     string
	ClientSecret []byte
}

type Proxy struct {
	HttpProxy    string
	HttpUser     string
	HttpPassword string
	HttpHost     string
	HttpPort     string

	HttpsProxy    string
	HttpsUser     string
	HttpsPassword string
	HttpsHost     string
	HttpsPort     string

	NoProxy          string
	TrustedCAMapName string
}
