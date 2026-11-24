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

package clusterapi

import (
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	api *ClusterAPI
)

type ClusterAPI struct {
	Client                  client.Client
	NonCachingClient        client.Client
	DiscoveryClient         discovery.DiscoveryInterface
	Scheme                  *runtime.Scheme
	ClientWrapper           clientwrapper.ClientWrapper
	NonCachingClientWrapper clientwrapper.ClientWrapper
}

func Initialize(
	client client.Client,
	nonCachingClient client.Client,
	discoveryClient discovery.DiscoveryInterface,
	scheme *runtime.Scheme,
) {
	api = &ClusterAPI{
		Client:                  client,
		ClientWrapper:           clientwrapper.New(client, scheme),
		NonCachingClient:        nonCachingClient,
		NonCachingClientWrapper: clientwrapper.New(nonCachingClient, scheme),
		DiscoveryClient:         discoveryClient,
		Scheme:                  scheme,
	}
}

func Get() *ClusterAPI {
	if api == nil {
		panic("ClusterAPI is not initialized")
	}
	return api
}
