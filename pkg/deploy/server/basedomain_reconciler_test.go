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

package server

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBaseDomainFromNetworkingDomain(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "my-domain.com",
			},
		},
	}).Build()

	reconciler := NewBaseDomainReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, "my-domain.com", ctx.CheCluster.Status.WorkspaceBaseDomain)
}

func TestBaseDomainFromExtraProperties(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "default-domain.com",
			},
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					ExtraProperties: map[string]string{
						"CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX": "custom-domain.com",
					},
				},
			},
		},
	}).Build()

	reconciler := NewBaseDomainReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, "custom-domain.com", ctx.CheCluster.Status.WorkspaceBaseDomain)
}

func TestBaseDomainExtraPropertiesOverridesNetworkingDomain(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "networking-domain.com",
			},
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					ExtraProperties: map[string]string{
						"CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX": "extra-domain.com",
					},
				},
			},
		},
	}).Build()

	reconciler := NewBaseDomainReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, "extra-domain.com", ctx.CheCluster.Status.WorkspaceBaseDomain)
}

func TestBaseDomainStatusUpdated(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "new-domain.com",
			},
		},
		Status: chev2.CheClusterStatus{
			WorkspaceBaseDomain: "old-domain.com",
		},
	}).Build()

	reconciler := NewBaseDomainReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, "new-domain.com", ctx.CheCluster.Status.WorkspaceBaseDomain)

	// Verify status was persisted
	cheCluster := &chev2.CheCluster{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"}, cheCluster)
	assert.Nil(t, err)
	assert.Equal(t, "new-domain.com", cheCluster.Status.WorkspaceBaseDomain)
}

func TestBaseDomainFromRoute(t *testing.T) {
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-che-test",
			Namespace: "eclipse-che",
		},
		Spec: routev1.RouteSpec{
			Host: "devworkspace-che-test.eclipse.org",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(route).Build()

	reconciler := NewBaseDomainReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.True(t, done)
	assert.Nil(t, err)
	assert.Equal(t, "eclipse.org", ctx.CheCluster.Status.WorkspaceBaseDomain)
}
