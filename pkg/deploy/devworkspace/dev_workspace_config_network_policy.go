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
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	dwoconfig "github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func updateNetworkPolicy(cheCtx *chetypes.CheContext, workspaceConfig *controllerv1alpha1.WorkspaceConfig) error {
	workspaceConfig.NetworkPolicy = getNetworkPolicyConfigFromSpec(cheCtx)

	dwoIngressRules, dwoEgressRules, err := dwoconfig.GetDefaultNetworkPolicy(cheCtx.DWONamespace)
	if err != nil {
		return err
	}

	if workspaceConfig.NetworkPolicy.Ingress == nil {
		workspaceConfig.NetworkPolicy.Ingress = append(dwoIngressRules, buildAllowFromCheIngressRule(cheCtx.CheCluster.Namespace))
	}

	if workspaceConfig.NetworkPolicy.Egress == nil {
		workspaceConfig.NetworkPolicy.Egress = dwoEgressRules
	}

	return nil
}

func getNetworkPolicyConfigFromSpec(cheCtx *chetypes.CheContext) *controllerv1alpha1.NetworkPolicyConfig {
	if cheCtx.CheCluster.Spec.DevEnvironments.Networking != nil &&
		cheCtx.CheCluster.Spec.DevEnvironments.Networking.NetworkPolicy != nil {

		return cheCtx.CheCluster.Spec.DevEnvironments.Networking.NetworkPolicy.DeepCopy()
	}

	return &controllerv1alpha1.NetworkPolicyConfig{}
}

func buildAllowFromCheIngressRule(namespace string) networkingv1.NetworkPolicyIngressRule {
	return networkingv1.NetworkPolicyIngressRule{
		From: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg},
				},
			},
		},
	}
}
