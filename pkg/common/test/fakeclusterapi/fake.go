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

package fakeclusterapi

import (
	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type Option func(*config)

type config struct {
	scheme             *runtime.Scheme
	statusSubresources []client.Object
	discoveryResources []*metav1.APIResourceList
}

func WithScheme(scheme *runtime.Scheme) Option {
	return func(c *config) { c.scheme = scheme }
}

func WithStatusSubresources(objs ...client.Object) Option {
	return func(c *config) { c.statusSubresources = objs }
}

func WithDiscoveryResources(resources ...*metav1.APIResourceList) Option {
	return func(c *config) { c.discoveryResources = resources }
}

func Setup(opts ...Option) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	scheme := cfg.scheme
	if scheme == nil {
		scheme = runtime.NewScheme()
	}

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(cfg.statusSubresources) > 0 {
		builder = builder.WithStatusSubresource(cfg.statusSubresources...)
	}
	fakeClient := builder.Build()

	clientSet := fakeclientset.NewClientset()
	discoveryClient, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		panic("failed to setup ClusterAPI")
	}
	discoveryClient.Resources = cfg.discoveryResources

	clusterapi.Initialize(fakeClient, fakeClient, discoveryClient, scheme)
}
