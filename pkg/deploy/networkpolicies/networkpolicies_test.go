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

package networkpolicies

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestReconcileNetworkPolicies(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				NetworkPolicies: &chev2.NetworkPolicies{Enabled: ptr.To(true)},
			},
		},
	}).Build()

	reconciler := NewNetworkPoliciesReconciler()
	_, done, err := reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	networkPolicy := &networkingv1.NetworkPolicy{}

	err = ctx.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "allow-from-workspaces-namespaces",
			Namespace: "eclipse-che",
		},
		networkPolicy,
	)
	assert.NoError(t, err)

	err = ctx.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "allow-from-same-namespace",
			Namespace: "eclipse-che",
		},
		networkPolicy,
	)
	assert.NoError(t, err)

	ctx.CheCluster.Spec.Networking.NetworkPolicies.Enabled = ptr.To(false)

	_, done, err = reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	err = ctx.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "allow-from-workspaces-namespaces",
			Namespace: "eclipse-che",
		},
		networkPolicy,
	)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	err = ctx.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "allow-from-same-namespace",
			Namespace: "eclipse-che",
		},
		networkPolicy,
	)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
