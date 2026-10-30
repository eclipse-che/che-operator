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
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NetworkPoliciesReconciler struct{}

func (r *NetworkPoliciesReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if cheCtx.CheCluster.IsNetworkPoliciesEnabled() {
		if err := r.syncAll(cheCtx); err != nil {
			return reconcile.Result{},
				false,
				fmt.Errorf("failed to reconcile NetworkPolicy: %w", err)
		}

		return reconcile.Result{}, true, nil
	}

	if err := r.deleteAll(cheCtx); err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to reconcile NetworkPolicy: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *NetworkPoliciesReconciler) Finalize(_ *chetypes.CheContext) error {
	return nil
}

func (r *NetworkPoliciesReconciler) syncAll(cheCtx *chetypes.CheContext) error {
	networkPolicies, err := r.buildNetworkPolicies(cheCtx)
	if err != nil {
		return err
	}

	for _, networkPolicy := range networkPolicies {
		if err := resources.NetworkPolicy.Sync(cheCtx, networkPolicy); err != nil {
			return err
		}
	}

	return nil
}

func (r *NetworkPoliciesReconciler) deleteAll(cheCtx *chetypes.CheContext) error {
	return resources.NetworkPolicy.DeleteAll(cheCtx, cheCtx.CheCluster.Namespace)
}

func (r *NetworkPoliciesReconciler) buildNetworkPolicies(cheCtx *chetypes.CheContext) ([]*networkingv1.NetworkPolicy, error) {
	if infrastructure.IsOpenShift() {
		return r.buildOpenShiftNetworkPolicies(cheCtx)
	}

	return r.buildKubernetesNetworkPolicies(cheCtx)
}
