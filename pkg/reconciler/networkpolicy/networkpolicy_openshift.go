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

package networkpolicy

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *NetworkPoliciesReconciler) buildOpenShiftNetworkPolicies(cheCtx *chetypes.CheContext) ([]*networkingv1.NetworkPolicy, error) {
	operatorNamespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	allowFromSameNamespace := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-same-namespace",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromOpenShiftIngress := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-openshift-ingress",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"network.openshift.io/policy-group": "ingress"},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromOpenShiftHostNetwork := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-openshift-host-network",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"policy-group.network.openshift.io/host-network": ""},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromOpenShiftMonitoring := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-openshift-monitoring",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"network.openshift.io/policy-group": "monitoring"},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromWorkspaces := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-workspaces",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{constants.KubernetesComponentLabelKey: constants.WorkspacesNamespaceComponentName},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromCheOperator := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("allow-from-%s-operator", defaults.GetCheFlavor()),
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": operatorNamespace},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowAllEgress := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-egress",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	return []*networkingv1.NetworkPolicy{
		allowFromSameNamespace,
		allowFromOpenShiftIngress,
		allowFromOpenShiftHostNetwork,
		allowFromOpenShiftMonitoring,
		allowFromWorkspaces,
		allowFromCheOperator,
		allowAllEgress,
	}, nil
}
