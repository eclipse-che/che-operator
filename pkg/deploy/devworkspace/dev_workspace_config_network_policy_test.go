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

package devworkspace

import (
	"slices"
	"testing"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	dwoconfig "github.com/devfile/devworkspace-operator/pkg/config"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDWONetworkPolicyRules(t *testing.T) {
	customIngressRule := networkingv1.NetworkPolicyIngressRule{
		From: []networkingv1.NetworkPolicyPeer{
			{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "custom-ingress"}}},
		},
	}

	existedDWOC := &controllerv1alpha1.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devWorkspaceConfigName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DevWorkspaceOperatorConfig",
			APIVersion: controllerv1alpha1.GroupVersion.String(),
		},
		Config: &controllerv1alpha1.OperatorConfiguration{
			Workspace: &controllerv1alpha1.WorkspaceConfig{
				NetworkPolicy: &controllerv1alpha1.NetworkPolicyConfig{
					Enabled: new(true),
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "stale-ingress"}}},
							},
						},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							To: []networkingv1.NetworkPolicyPeer{
								{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "stale-egress"}}},
							},
						},
					},
				},
			},
		},
	}

	defaultIngressRules, defaultEgressRules, err := dwoconfig.GetDefaultNetworkPolicy("devworkspace-controller")
	assert.NoError(t, err)

	type testCase struct {
		name                    string
		dwoNetworkPolicyConfig  *controllerv1alpha1.NetworkPolicyConfig
		existedObjects          []client.Object
		expectedDWOIngressRules []networkingv1.NetworkPolicyIngressRule
		expectedDWOEgressRules  []networkingv1.NetworkPolicyEgressRule
	}

	testCases := []testCase{
		{
			name:                    "Set default rules and the rule allowing traffic from Che operands when the network policy is not set",
			dwoNetworkPolicyConfig:  nil,
			expectedDWOIngressRules: slices.Concat(defaultIngressRules, []networkingv1.NetworkPolicyIngressRule{buildAllowFromCheIngressRule("eclipse-che")}),
			expectedDWOEgressRules:  defaultEgressRules,
		},
		{
			name:                    "Set default rules and the rule allowing traffic from Che operands when the network policy is empty",
			dwoNetworkPolicyConfig:  &controllerv1alpha1.NetworkPolicyConfig{},
			expectedDWOIngressRules: slices.Concat(defaultIngressRules, []networkingv1.NetworkPolicyIngressRule{buildAllowFromCheIngressRule("eclipse-che")}),
			expectedDWOEgressRules:  defaultEgressRules,
		},
		{
			name: "Keep custom ingress rules and not add the rule allowing traffic from Che operands",
			dwoNetworkPolicyConfig: &controllerv1alpha1.NetworkPolicyConfig{
				Ingress: []networkingv1.NetworkPolicyIngressRule{customIngressRule},
			},
			expectedDWOIngressRules: []networkingv1.NetworkPolicyIngressRule{customIngressRule},
			expectedDWOEgressRules:  defaultEgressRules,
		},
		{
			name: "Don't Lose the empty rule lists when they are written to the DWOC",
			dwoNetworkPolicyConfig: &controllerv1alpha1.NetworkPolicyConfig{
				Ingress: []networkingv1.NetworkPolicyIngressRule{},
				Egress:  []networkingv1.NetworkPolicyEgressRule{},
			},
			expectedDWOIngressRules: []networkingv1.NetworkPolicyIngressRule{},
			expectedDWOEgressRules:  []networkingv1.NetworkPolicyEgressRule{},
		},
		{
			name:                    "Override the network policy already set in the DWOC with the default rules when the network policy is not set",
			dwoNetworkPolicyConfig:  nil,
			existedObjects:          []client.Object{existedDWOC},
			expectedDWOIngressRules: slices.Concat(defaultIngressRules, []networkingv1.NetworkPolicyIngressRule{buildAllowFromCheIngressRule("eclipse-che")}),
			expectedDWOEgressRules:  defaultEgressRules,
		},
		{
			name:                    "Override the network policy already set in the DWOC with the default rules when the network policy is empty",
			dwoNetworkPolicyConfig:  &controllerv1alpha1.NetworkPolicyConfig{},
			existedObjects:          []client.Object{existedDWOC},
			expectedDWOIngressRules: slices.Concat(defaultIngressRules, []networkingv1.NetworkPolicyIngressRule{buildAllowFromCheIngressRule("eclipse-che")}),
			expectedDWOEgressRules:  defaultEgressRules,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cheCtx := test.NewCtxBuilder().WithCheCluster(
				&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						DevEnvironments: chev2.CheClusterDevEnvironments{
							Networking: &chev2.DevEnvironmentNetworking{NetworkPolicy: testCase.dwoNetworkPolicyConfig},
						},
					},
				}).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, cheCtx, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := cheCtx.ClusterAPI.Client.Get(
				cheCtx.Context,
				types.NamespacedName{Name: devWorkspaceConfigName, Namespace: "eclipse-che"},
				dwoc)
			assert.NoError(t, err)

			assert.NotNil(t, dwoc.Config)
			assert.NotNil(t, dwoc.Config.Workspace)

			dwocNetworkPolicy := dwoc.Config.Workspace.NetworkPolicy

			assert.NotNil(t, dwocNetworkPolicy)
			assert.Equal(t, testCase.expectedDWOIngressRules, dwocNetworkPolicy.Ingress)
			assert.Equal(t, testCase.expectedDWOEgressRules, dwocNetworkPolicy.Egress)
		})
	}
}

func TestDWONetworkPolicyEnabled(t *testing.T) {
	type testCase struct {
		name                            string
		dwoNetworkPolicyConfig          *controllerv1alpha1.NetworkPolicyConfig
		cheNetworkPolicyConfig          *chev2.NetworkPolicy
		expectedDWONetworkPolicyEnabled *bool
	}

	testCases := []testCase{
		{
			name: "Do not set the DWO network policy enabled flag when it is not set in the Che or the DWO network policy configuration",
			cheNetworkPolicyConfig: &chev2.NetworkPolicy{
				Enabled: nil,
			},
			dwoNetworkPolicyConfig: &controllerv1alpha1.NetworkPolicyConfig{
				Enabled: nil,
			},
			expectedDWONetworkPolicyEnabled: nil,
		},
		{
			name: "Do not set the DWO network policy enabled flag when the Che network policy is disabled",
			cheNetworkPolicyConfig: &chev2.NetworkPolicy{
				Enabled: new(false),
			},
			dwoNetworkPolicyConfig: &controllerv1alpha1.NetworkPolicyConfig{
				Enabled: nil,
			},
			expectedDWONetworkPolicyEnabled: nil,
		},
		{
			name: "Set the DWO network policy enabled flag when the Che network policy is enabled",
			cheNetworkPolicyConfig: &chev2.NetworkPolicy{
				Enabled: new(true),
			},
			dwoNetworkPolicyConfig: &controllerv1alpha1.NetworkPolicyConfig{
				Enabled: nil,
			},
			expectedDWONetworkPolicyEnabled: new(true),
		},
		{
			name: "Keep the DWO network policy enabled flag set in the dev environments networking configuration",
			dwoNetworkPolicyConfig: &controllerv1alpha1.NetworkPolicyConfig{
				Enabled: new(true),
			},
			expectedDWONetworkPolicyEnabled: new(true),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cheCtx := test.NewCtxBuilder().WithCheCluster(
				&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Networking: chev2.CheClusterSpecNetworking{
							NetworkPolicy: testCase.cheNetworkPolicyConfig,
						},
						DevEnvironments: chev2.CheClusterDevEnvironments{
							Networking: &chev2.DevEnvironmentNetworking{NetworkPolicy: testCase.dwoNetworkPolicyConfig},
						},
					},
				}).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, cheCtx, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := cheCtx.ClusterAPI.Client.Get(
				cheCtx.Context,
				types.NamespacedName{Name: devWorkspaceConfigName, Namespace: "eclipse-che"},
				dwoc)
			assert.NoError(t, err)

			assert.NotNil(t, dwoc.Config)
			assert.NotNil(t, dwoc.Config.Workspace)
			assert.NotNil(t, dwoc.Config.Workspace.NetworkPolicy)
			assert.Equal(t, testCase.expectedDWONetworkPolicyEnabled, dwoc.Config.Workspace.NetworkPolicy.Enabled)
		})
	}
}
