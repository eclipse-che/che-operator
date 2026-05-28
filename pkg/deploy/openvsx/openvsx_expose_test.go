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

package openvsx

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileCreatesIngress(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://eclipse-che.apps.cluster.example.com",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXExposeReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: OpenVSXIngressName, Namespace: "eclipse-che"}, &networking.Ingress{}))
	assert.Equal(t, "https://openvsx-eclipse-che.apps.cluster.example.com", ctx.CheCluster.Status.OpenVSXURL)
}

func TestReconcileDeletesIngressWhenDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://eclipse-che.apps.cluster.example.com",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXExposeReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: OpenVSXIngressName, Namespace: "eclipse-che"}, &networking.Ingress{}))

	ctx.CheCluster.Spec.Components.OpenVSX.Enable = false
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: OpenVSXIngressName, Namespace: "eclipse-che"}, &networking.Ingress{}))
	assert.Empty(t, ctx.CheCluster.Status.OpenVSXURL)
}

func TestIngressSpec(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://eclipse-che.apps.cluster.example.com",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSX: chev2.OpenVSX{
					Enable: true,
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXExposeReconciler()
	hostname := reconciler.getHostname(ctx)
	ingress := reconciler.getIngressSpec(ctx, hostname)

	assert.Equal(t, "openvsx-eclipse-che.apps.cluster.example.com", hostname)
	assert.Equal(t, OpenVSXIngressName, ingress.Name)
	assert.Equal(t, "eclipse-che", ingress.Namespace)

	rule := ingress.Spec.Rules[0]
	assert.Equal(t, hostname, rule.Host)

	paths := rule.HTTP.Paths
	assert.Equal(t, len(serverPaths)+1, len(paths))

	for i, sp := range serverPaths {
		assert.Equal(t, sp, paths[i].Path)
		assert.Equal(t, constants.OpenVSXServerName, paths[i].Backend.Service.Name)
		assert.Equal(t, int32(8080), paths[i].Backend.Service.Port.Number)
	}

	webuiPath := paths[len(paths)-1]
	assert.Equal(t, "/", webuiPath.Path)
	assert.Equal(t, constants.OpenVSXWebUIName, webuiPath.Backend.Service.Name)
	assert.Equal(t, int32(3000), webuiPath.Backend.Service.Port.Number)

	assert.Equal(t, hostname, ingress.Spec.TLS[0].Hosts[0])
}

func TestHostnameDerivation(t *testing.T) {
	reconciler := NewOpenVSXExposeReconciler()

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://eclipse-che.apps.cluster.example.com",
		},
	}).Build()

	assert.Equal(t, "openvsx-eclipse-che.apps.cluster.example.com", reconciler.getHostname(ctx))
}

func TestHostnameFromDomain(t *testing.T) {
	reconciler := NewOpenVSXExposeReconciler()

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "my-namespace",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "apps.cluster.example.com",
			},
		},
	}).Build()

	assert.Equal(t, "openvsx-my-namespace.apps.cluster.example.com", reconciler.getHostname(ctx))
}

func TestHostnameEmpty(t *testing.T) {
	reconciler := NewOpenVSXExposeReconciler()

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
	}).Build()

	assert.Empty(t, reconciler.getHostname(ctx))
}
