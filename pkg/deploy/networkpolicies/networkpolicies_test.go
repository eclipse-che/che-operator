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
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var allNetworkPolicyNames = []string{
	"allow-from-same-namespace",
	"allow-from-workspaces",
	"allow-from-openshift-ingress",
	"allow-from-openshift-monitoring",
	"allow-from-operator",
	"allow-all-egress",
}

func buildCtx(networkPolicyEnabled bool) *test.DeployContextBuild {
	return test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Networking: chev2.CheClusterSpecNetworking{
					NetworkPolicy: &chev2.NetworkPolicy{Enabled: ptr.To(networkPolicyEnabled)},
				},
			},
		})
}

func TestReconcileNetworkPoliciesCreatesAllPolicies(t *testing.T) {
	ctx := buildCtx(true).Build()
	infrastructure.SetOperatorNamespaceForTesting("eclipse-che")

	reconciler := NewNetworkPoliciesReconciler()

	_, done, err := reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	npList := &networkingv1.NetworkPolicyList{}
	err = ctx.ClusterAPI.Client.List(context.TODO(), npList, &client.ListOptions{Namespace: "eclipse-che"})
	assert.NoError(t, err)
	assert.Equal(t, len(allNetworkPolicyNames), len(npList.Items))

	for _, name := range allNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := ctx.ClusterAPI.Client.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "eclipse-che"},
			np,
		)
		assert.NoError(t, err)
	}
}

func TestReconcileNetworkPoliciesDeletesWhenDisabled(t *testing.T) {
	ctx := buildCtx(true).Build()
	infrastructure.SetOperatorNamespaceForTesting("eclipse-che")

	reconciler := NewNetworkPoliciesReconciler()

	_, done, err := reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	ctx.CheCluster.Spec.Networking.NetworkPolicy.Enabled = ptr.To(false)

	_, done, err = reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	for _, name := range allNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := ctx.ClusterAPI.Client.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "eclipse-che"},
			np,
		)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcileNetworkPoliciesIdempotent(t *testing.T) {
	ctx := buildCtx(true).Build()
	infrastructure.SetOperatorNamespaceForTesting("eclipse-che")

	reconciler := NewNetworkPoliciesReconciler()

	_, done, err := reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	_, done, err = reconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	for _, name := range allNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := ctx.ClusterAPI.Client.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "eclipse-che"},
			np,
		)
		assert.NoError(t, err)
	}
}
