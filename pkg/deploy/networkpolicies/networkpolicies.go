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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
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
		networkPolicyList := &networkingv1.NetworkPolicyList{}

		items, err := ctx.ClusterAPI.ClientWrapper.List(context.TODO(), networkPolicyList,
			&client.ListOptions{
				Namespace:     ctx.CheCluster.Namespace,
				LabelSelector: labels.SelectorFromSet(deploy.GetLabels(defaults.GetCheFlavor())),
			})
		if err != nil {
			return reconcile.Result{}, false, fmt.Errorf("could not list NetworkPolicy objects in namespace %s: %w", ctx.CheCluster.Namespace, err)
		}

		for _, item := range items {
			networkPolicy, ok := item.(*networkingv1.NetworkPolicy)
			if !ok {
				continue
			}

			err = ctx.ClusterAPI.ClientWrapper.DeleteIgnoreNotFound(context.TODO(), networkPolicy)
			if err != nil {
				return reconcile.Result{}, false, fmt.Errorf("failed to delete NetworkPolicy %s/%s: %w", networkPolicy.GetNamespace(), networkPolicy.GetName(), err)
			}
		}

		return reconcile.Result{}, true, nil
	}

	err := r.syncNetworkPolicy(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync NetworkPolicy: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *NetworkPoliciesReconciler) Finalize(_ *chetypes.DeployContext) bool {
	return true
}

func (r *NetworkPoliciesReconciler) syncNetworkPolicy(ctx *chetypes.DeployContext) error {
	networkPolicies := r.getNetworkPolicies(ctx)

	for _, networkPolicy := range networkPolicies {
		if err := controllerutil.SetControllerReference(ctx.CheCluster, networkPolicy, ctx.ClusterAPI.Scheme); err != nil {
			return fmt.Errorf("could not set controller reference: %w", err)
		}

		if err := ctx.ClusterAPI.ClientWrapper.Sync(
			context.TODO(),
			networkPolicy,
			&k8sclient.SyncOptions{DiffOpts: diffs.NetworkPolicy},
		); err != nil {
			return fmt.Errorf("failed to sync NetworkPolicy %s/%s: %w", networkPolicy.Namespace, networkPolicy.Name, err)
		}
	}

	return nil
}

func (r *NetworkPoliciesReconciler) getNetworkPolicies(ctx *chetypes.DeployContext) []*networkingv1.NetworkPolicy {
	allowFromSameNamespaceNetworkPolicy := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-same-namespace",
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	allowFromWorkspacesNamespacesNetworkPolicy := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-workspaces-namespaces",
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
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

	return []*networkingv1.NetworkPolicy{
		allowFromSameNamespaceNetworkPolicy,
		allowFromWorkspacesNamespacesNetworkPolicy,
	}
}
