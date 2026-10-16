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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *NetworkPoliciesReconciler) buildKubernetesNetworkPolicies(cheCtx *chetypes.CheContext) ([]*networkingv1.NetworkPolicy, error) {
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

	allowAllIngress := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-ingress",
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
			},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	return []*networkingv1.NetworkPolicy{
		allowAllEgress,
		allowAllIngress,
	}, nil
}
