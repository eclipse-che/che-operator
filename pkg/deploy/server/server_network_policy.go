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
	"fmt"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	allowFromWorkspacesNamespacesPolicy = "allow-from-workspaces-namespaces"
)

func (s *CheServerReconciler) syncNetworkPolicies(ctx *chetypes.DeployContext) (bool, error) {
	isNetworkPolicyEnabled := ctx.CheCluster.Spec.Networking.NetworkPolicies != nil &&
		ctx.CheCluster.Spec.Networking.NetworkPolicies.Enabled

	if !isNetworkPolicyEnabled {
		networkPolicy := &networkingv1.NetworkPolicy{}
		exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{
				Name:      allowFromWorkspacesNamespacesPolicy,
				Namespace: ctx.CheCluster.Namespace,
			},
			networkPolicy,
		)
		if err != nil {
			return false, fmt.Errorf("failed to get NetworkPolicy %s/%s: %w", allowFromWorkspacesNamespacesPolicy, ctx.CheCluster.Namespace, err)
		}
		if !exists {
			return true, nil
		}

		// Ensures that NetworkPolicy was created by operator
		if deploy.IsPartOfEclipseCheAndManagedByOperator(networkPolicy.GetLabels(), defaults.GetCheFlavor()) {
			err = ctx.ClusterAPI.ClientWrapper.DeleteIgnoreNotFound(context.TODO(), networkPolicy)
			if err != nil {
				return false, fmt.Errorf("failed to delete NetworkPolicy %s/%s: %w", allowFromWorkspacesNamespacesPolicy, ctx.CheCluster.Namespace, err)
			}
		}

		return true, nil
	}

	policy := networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      allowFromWorkspacesNamespacesPolicy,
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
									constants.KubernetesComponentLabelKey: namespacecache.CheComponentLabelValue,
								},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, &policy, ctx.ClusterAPI.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference on network policy %s/%s: %w", ctx.CheCluster.Namespace, allowFromWorkspacesNamespacesPolicy, err)
	}

	if err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		&policy,
		&k8sclient.SyncOptions{DiffOpts: diffs.NetworkPolicy},
	); err != nil {
		return false, fmt.Errorf("failed to sync network policy %s/%s: %w", ctx.CheCluster.Namespace, allowFromWorkspacesNamespacesPolicy, err)
	}

	return true, nil
}
