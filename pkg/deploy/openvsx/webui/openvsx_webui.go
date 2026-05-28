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

package webui

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXWebUIReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXWebUIReconciler() *OpenVSXWebUIReconciler {
	return &OpenVSXWebUIReconciler{}
}

func (r *OpenVSXWebUIReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsOpenVSXOperandEnabled() {
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXWebUIName, &appsv1.Deployment{})
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXWebUIName, &corev1.Service{})
		return reconcile.Result{}, true, nil
	}

	done, err := r.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXWebUIReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (r *OpenVSXWebUIReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.OpenVSXWebUIName,
		[]string{"http"},
		[]int32{3000},
		constants.OpenVSXWebUIName)
}

func (r *OpenVSXWebUIReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := r.getDeploymentSpec(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}
