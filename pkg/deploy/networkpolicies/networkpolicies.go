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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NetworkPoliciesReconciler struct {
	reconciler.Reconcilable
}

func NewNetworkPoliciesReconciler() *NetworkPoliciesReconciler {
	return &NetworkPoliciesReconciler{}
}

func (r *NetworkPoliciesReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsNetworkPoliciesEnabled() {
		err := DeleteNetworkPolicy(ctx, ctx.CheCluster.Namespace)
		if err != nil {
			err = fmt.Errorf("failed to delete NetworkPolicy in namespace %s: %w", ctx.CheCluster.Namespace, err)
		}

		return reconcile.Result{}, err == nil, err
	}

	err := SyncNetworkPolicy(ctx, ctx.CheCluster.Namespace)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync NetworkPolicy in namespace %s: %w", ctx.CheCluster.Namespace, err)
	}

	return reconcile.Result{}, true, nil
}

func (r *NetworkPoliciesReconciler) Finalize(_ *chetypes.DeployContext) bool {
	return true
}

func GetNetworkPolicies(ctx *chetypes.DeployContext, namespace string) ([]*networkingv1.NetworkPolicy, error) {
	isWorkspaceNetworkPolicies := ctx.CheCluster.Namespace != namespace

	operatorNamespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator namespace: %w", err)
	}

	var podSelector metav1.LabelSelector

	if isWorkspaceNetworkPolicies {
		podSelector = metav1.LabelSelector{}
	} else {
		podSelector = metav1.LabelSelector{
			MatchLabels: map[string]string{
				constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
			},
		}
	}

	allowFromSameNamespace := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-same-namespace",
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &podSelector,
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
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
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
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
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
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
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
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromChe := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-" + ctx.CheCluster.Namespace,
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": ctx.CheCluster.Namespace,
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
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": operatorNamespace,
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
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromDevWorkspaceOperator := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-devworkspace-operator",
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": ctx.DWONamespace,
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
			Namespace: namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	networkPolicies := []*networkingv1.NetworkPolicy{
		allowFromSameNamespace,
		allowFromOpenShiftIngress,
		allowFromOpenShiftMonitoring,
		allowAllEgress,
	}

	if isWorkspaceNetworkPolicies {
		networkPolicies = append(networkPolicies, allowFromChe, allowFromDevWorkspaceOperator)
	} else {
		networkPolicies = append(networkPolicies, allowFromWorkspaces, allowFromCheOperator)
	}

	return networkPolicies, nil
}

func SyncNetworkPolicy(ctx *chetypes.DeployContext, namespace string) error {
	networkPolicies, err := GetNetworkPolicies(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to get NetworkPolicy in namespace %s: %w", namespace, err)
	}

	for _, networkPolicy := range networkPolicies {
		if ctx.CheCluster.Namespace == namespace {
			if err = controllerutil.SetControllerReference(ctx.CheCluster, networkPolicy, ctx.ClusterAPI.Scheme); err != nil {
				return err
			}
		}

		if err := ctx.ClusterAPI.ClientWrapper.Sync(
			ctx.Context,
			networkPolicy,
			&k8sclient.SyncOptions{DiffOpts: diffs.NetworkPolicy},
		); err != nil {
			return fmt.Errorf("failed to sync NetworkPolicy %s/%s: %w", networkPolicy.Namespace, networkPolicy.Name, err)
		}
	}

	return nil
}

func DeleteNetworkPolicy(ctx *chetypes.DeployContext, namespace string) error {
	items, err := ctx.ClusterAPI.ClientWrapper.List(
		ctx.Context,
		&networkingv1.NetworkPolicyList{},
		&client.ListOptions{
			Namespace:     namespace,
			LabelSelector: labels.SelectorFromSet(deploy.GetLabels(defaults.GetCheFlavor())),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to list NetworkPolicy in namespace %s: %w", namespace, err)
	}

	for _, item := range items {
		networkPolicy, ok := item.(*networkingv1.NetworkPolicy)
		if !ok {
			continue
		}

		err = ctx.ClusterAPI.ClientWrapper.DeleteIgnoreNotFound(ctx.Context, networkPolicy)
		if err != nil {
			return fmt.Errorf("failed to delete NetworkPolicy %s/%s: %w", networkPolicy.GetNamespace(), networkPolicy.GetName(), err)
		}
	}

	return nil
}
