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
	"slices"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var cheClusterPolicyNames = []string{
	"allow-from-same-namespace",
	"allow-from-openshift-ingress",
	"allow-from-openshift-monitoring",
	"allow-from-workspaces",
	"allow-from-che-operator",
	"allow-all-egress",
}

var workspacePolicyNames = []string{
	"allow-from-same-namespace",
	"allow-from-openshift-ingress",
	"allow-from-openshift-monitoring",
	"allow-from-eclipse-che",
	"allow-from-devworkspace-operator",
	"allow-all-egress",
}

func TestReconcileCreatesNetworkPoliciesWhenEnabled(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()
	ctx.CheCluster.Spec.Networking.NetworkPolicy = &chev2.NetworkPolicy{Enabled: ptr.To(true)}

	reconciler := NewNetworkPoliciesReconciler()

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	for _, name := range cheClusterPolicyNames {
		exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: ctx.CheCluster.Namespace},
			&networkingv1.NetworkPolicy{},
		)

		assert.NoError(t, err)
		assert.True(t, exists)
	}
}

func TestReconcileDeletesNetworkPoliciesWhenDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()
	ctx.CheCluster.Spec.Networking.NetworkPolicy = &chev2.NetworkPolicy{Enabled: ptr.To(true)}

	reconciler := NewNetworkPoliciesReconciler()

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	ctx.CheCluster.Spec.Networking.NetworkPolicy.Enabled = ptr.To(false)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	for _, name := range cheClusterPolicyNames {
		exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: ctx.CheCluster.Namespace},
			&networkingv1.NetworkPolicy{},
		)

		assert.NoError(t, err)
		assert.False(t, exists)
	}
}

func TestReconcileDeletesOnlyLabeledPolicies(t *testing.T) {
	unownedPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unowned-policy",
			Namespace: "eclipse-che",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(unownedPolicy).Build()
	ctx.CheCluster.Spec.Networking.NetworkPolicy = &chev2.NetworkPolicy{Enabled: ptr.To(true)}

	reconciler := NewNetworkPoliciesReconciler()

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	ctx.CheCluster.Spec.Networking.NetworkPolicy.Enabled = ptr.To(false)

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: "unowned-policy", Namespace: ctx.CheCluster.Namespace},
		&networkingv1.NetworkPolicy{},
	)

	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestReconcileIdempotent(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()
	ctx.CheCluster.Spec.Networking.NetworkPolicy = &chev2.NetworkPolicy{Enabled: ptr.To(true)}

	reconciler := NewNetworkPoliciesReconciler()

	test.EnsureReconcile(t, ctx, reconciler.Reconcile)
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	for _, name := range cheClusterPolicyNames {
		exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: ctx.CheCluster.Namespace},
			&networkingv1.NetworkPolicy{},
		)

		assert.NoError(t, err)
		assert.True(t, exists)
	}
}

func TestGetNetworkPoliciesPodSelectorForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policies, err := GetNetworkPolicies(ctx, ctx.CheCluster.Namespace)
	assert.NoError(t, err)

	expectedSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
		},
	}

	for _, p := range policies {
		assert.Equal(t, expectedSelector, p.Spec.PodSelector)
	}
}

func TestGetNetworkPoliciesPodSelectorForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policies, err := GetNetworkPolicies(ctx, "user-ns")
	assert.NoError(t, err)

	expectedSelector := metav1.LabelSelector{}

	for _, p := range policies {
		assert.Equal(t, expectedSelector, p.Spec.PodSelector)
	}
}

func TestGetNetworkPoliciesLabelsForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policies, err := GetNetworkPolicies(ctx, ctx.CheCluster.Namespace)
	assert.NoError(t, err)

	expectedLabels := deploy.GetLabels(defaults.GetCheFlavor())

	for _, p := range policies {
		assert.Equal(t, expectedLabels, p.Labels)
	}
}

func TestGetNetworkPoliciesLabelsForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policies, err := GetNetworkPolicies(ctx, "user-ns")
	assert.NoError(t, err)

	expectedLabels := deploy.GetLabels(defaults.GetCheFlavor())

	for _, p := range policies {
		assert.Equal(t, expectedLabels, p.Labels)
	}
}

func TestGetNetworkPoliciesNamespaceForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policies, err := GetNetworkPolicies(ctx, ctx.CheCluster.Namespace)
	assert.NoError(t, err)

	for _, p := range policies {
		assert.Equal(t, ctx.CheCluster.Namespace, p.Namespace)
	}
}

func TestGetNetworkPoliciesNamespaceForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policies, err := GetNetworkPolicies(ctx, "user-ns")
	assert.NoError(t, err)

	for _, p := range policies {
		assert.Equal(t, "user-ns", p.Namespace)
	}
}

func TestAllowFromSameNamespaceSpecForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, ctx.CheCluster.Namespace, "allow-from-same-namespace")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
					},
				}},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromSameNamespaceSpecForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, "user-ns", "allow-from-same-namespace")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{PodSelector: &metav1.LabelSelector{}},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromOpenShiftIngressSpecForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, ctx.CheCluster.Namespace, "allow-from-openshift-ingress")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"network.openshift.io/policy-group": "ingress",
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromOpenShiftIngressSpecForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, "user-ns", "allow-from-openshift-ingress")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"network.openshift.io/policy-group": "ingress",
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromOpenShiftMonitoringSpecForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, ctx.CheCluster.Namespace, "allow-from-openshift-monitoring")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"network.openshift.io/policy-group": "monitoring",
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromOpenShiftMonitoringSpecForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, "user-ns", "allow-from-openshift-monitoring")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"network.openshift.io/policy-group": "monitoring",
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromWorkspacesSpec(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, ctx.CheCluster.Namespace, "allow-from-workspaces")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constants.KubernetesComponentLabelKey: constants.WorkspacesNamespaceComponentName,
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromCheOperatorSpec(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, ctx.CheCluster.Namespace, "allow-from-che-operator")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "openshift-operators",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromCheNamespaceSpec(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, "user-ns", "allow-from-eclipse-che")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "eclipse-che",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowFromDevWorkspaceOperatorSpec(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, "user-ns", "allow-from-devworkspace-operator")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "devworkspace-controller",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
						},
					},
				},
			},
		},
	}, policy.Spec.Ingress)
}

func TestAllowAllEgressSpecForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, ctx.CheCluster.Namespace, "allow-all-egress")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, policy.Spec.PolicyTypes)
	assert.Empty(t, policy.Spec.Ingress)
	assert.Equal(t, []networkingv1.NetworkPolicyEgressRule{{}}, policy.Spec.Egress)
}

func TestAllowAllEgressSpecForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	policy := findPolicy(t, ctx, "user-ns", "allow-all-egress")

	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, policy.Spec.PolicyTypes)
	assert.Empty(t, policy.Spec.Ingress)
	assert.Equal(t, []networkingv1.NetworkPolicyEgressRule{{}}, policy.Spec.Egress)
}

func TestSyncNetworkPolicySetsOwnerReferenceForCheClusterNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := SyncNetworkPolicy(ctx, ctx.CheCluster.Namespace)
	assert.NoError(t, err)

	for _, name := range cheClusterPolicyNames {
		networkPolicy := &networkingv1.NetworkPolicy{}
		exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "eclipse-che"},
			networkPolicy,
		)

		assert.NoError(t, err)
		assert.True(t, exists)
		assert.NotEmpty(t, networkPolicy.OwnerReferences)
		assert.Equal(t, ctx.CheCluster.Name, networkPolicy.OwnerReferences[0].Name)
	}
}

func TestSyncNetworkPolicyNoOwnerReferenceForWorkspaceNamespace(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := SyncNetworkPolicy(ctx, "user-ns")
	assert.NoError(t, err)

	for _, name := range workspacePolicyNames {
		networkPolicy := &networkingv1.NetworkPolicy{}
		exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "user-ns"},
			networkPolicy,
		)

		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Empty(t, networkPolicy.OwnerReferences)
	}
}

func findPolicy(t *testing.T, ctx *chetypes.DeployContext, namespace string, name string) *networkingv1.NetworkPolicy {
	t.Helper()

	policies, err := GetNetworkPolicies(ctx, namespace)
	assert.NoError(t, err)

	idx := slices.IndexFunc(policies, func(item *networkingv1.NetworkPolicy) bool { return item.Name == name })
	if idx == -1 {
		t.Fatalf("policy %s not found in namespace %s", name, namespace)
	}

	return policies[idx]
}
