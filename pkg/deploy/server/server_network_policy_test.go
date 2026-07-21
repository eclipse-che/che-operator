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
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncNetworkPoliciesCreatesWhenEnabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				NetworkPolicies: &chev2.NetworkPolicies{Enabled: true},
			},
		},
	}).Build()

	server := NewCheServerReconciler()
	done, err := server.syncNetworkPolicies(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	key := types.NamespacedName{
		Name:      allowFromWorkspacesNamespacesPolicy,
		Namespace: "eclipse-che",
	}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), key, np)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, np.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, defaults.GetCheFlavor(), np.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t,
		constants.WorkspacesNamespaceComponentName,
		np.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels[constants.KubernetesComponentLabelKey],
	)
	assert.Equal(t, 1, len(np.OwnerReferences))
	assert.Equal(t, "eclipse-che", np.OwnerReferences[0].Name)
}

func TestSyncNetworkPoliciesSkipsWhenDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
	}).Build()

	server := NewCheServerReconciler()
	done, err := server.syncNetworkPolicies(ctx)
	assert.True(t, done)
	assert.NoError(t, err)

	assert.False(t, test.IsObjectExists(
		ctx.ClusterAPI.Client,
		types.NamespacedName{
			Name:      allowFromWorkspacesNamespacesPolicy,
			Namespace: "eclipse-che",
		},
		&networkingv1.NetworkPolicy{},
	))
}

func TestSyncNetworkPoliciesDeletesWhenToggleOff(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				NetworkPolicies: &chev2.NetworkPolicies{Enabled: true},
			},
		},
	}).Build()

	server := NewCheServerReconciler()
	done, err := server.syncNetworkPolicies(ctx)
	assert.True(t, done)
	assert.NoError(t, err)
	key := types.NamespacedName{
		Name:      allowFromWorkspacesNamespacesPolicy,
		Namespace: "eclipse-che",
	}
	assert.True(t, test.IsObjectExists(
		ctx.ClusterAPI.Client,
		key,
		&networkingv1.NetworkPolicy{},
	))

	ctx.CheCluster.Spec.Networking.NetworkPolicies.Enabled = false
	done, err = server.syncNetworkPolicies(ctx)
	assert.True(t, done)
	assert.NoError(t, err)
	assert.False(t, test.IsObjectExists(
		ctx.ClusterAPI.Client,
		key,
		&networkingv1.NetworkPolicy{},
	))
}
