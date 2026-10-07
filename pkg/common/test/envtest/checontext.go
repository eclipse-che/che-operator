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

package envtest

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheContextBuilder struct {
	env        *Env
	cheCluster *chev2.CheCluster
	initObject []client.Object
}

// NewCheCtxBuilder returns a builder of a CheContext backed by the envtest API server.
func (e *Env) NewCheCtxBuilder() *CheContextBuilder {
	return &CheContextBuilder{
		env:        e,
		initObject: []client.Object{},
	}
}

func (b *CheContextBuilder) WithObjects(initObjs ...client.Object) *CheContextBuilder {
	b.initObject = append(b.initObject, initObjs...)
	return b
}

func (b *CheContextBuilder) WithCheCluster(cheCluster *chev2.CheCluster) *CheContextBuilder {
	b.cheCluster = cheCluster
	return b
}

func (b *CheContextBuilder) WithEmptyCheCluster() *CheContextBuilder {
	b.cheCluster = &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
	}
	return b
}

// Build creates the configured objects (and the namespaces they live in) in the envtest API server and
// returns a CheContext wired to it.
//
// Both `Client`/`ClientWrapper` and `NonCachingClient`/`NonCachingClientWrapper` are backed by the same
// direct client - envtest runs no manager and therefore has no cache - so code that branches on cached
// vs non-caching  behaves identically on either side and that branch cannot be asserted on from tests.
func (b *CheContextBuilder) Build() *chetypes.CheContext {
	ginkgo.GinkgoHelper()

	ctx := b.env.Context

	if b.cheCluster != nil {
		b.initObject = append(b.initObject, b.cheCluster)
	}

	for _, obj := range b.initObject {
		gomega.Expect(b.env.ensureNamespaceExists(obj.GetNamespace())).To(gomega.Succeed())
		gomega.Expect(b.env.Client.Create(ctx, obj)).To(gomega.Succeed())
	}

	return &chetypes.CheContext{
		CheCluster: b.cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:                  b.env.Client,
			NonCachingClient:        b.env.Client,
			DiscoveryClient:         b.env.DiscoveryClient,
			Scheme:                  b.env.Scheme,
			ClientWrapper:           k8sclient.NewK8sClient(b.env.Client, b.env.Scheme),
			NonCachingClientWrapper: k8sclient.NewK8sClient(b.env.Client, b.env.Scheme),
		},
		Context: ctx,
	}
}
