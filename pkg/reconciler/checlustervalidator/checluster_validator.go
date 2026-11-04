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

package checlustervalidator

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CheClusterValidatorReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *CheClusterValidatorReconciler {
	return &CheClusterValidatorReconciler{}
}

func (v *CheClusterValidatorReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if !infrastructure.IsOpenShift() {
		return v.validateAll(cheCtx)
	}

	return reconcile.Result{}, true, nil
}

func (v *CheClusterValidatorReconciler) validateAll(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if cheCtx.CheCluster.Spec.Networking.Domain == "" {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to validate CheCluster %s/%s: required field \"spec.networking.domain\" is not set; set it to the base domain of the cluster ingress (e.g. the value of a DNS-1123 domain used to route traffic to the cluster)",
				cheCtx.CheCluster.Namespace, cheCtx.CheCluster.Name)
	}

	return reconcile.Result{}, false, nil
}

func (v *CheClusterValidatorReconciler) Finalize(_ *chetypes.CheContext) error {
	return nil
}
