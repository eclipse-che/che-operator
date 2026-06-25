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

package openvsx_database

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXDatabaseReconciler struct {
	reconciler.Reconcilable
}

var logger = ctrl.Log.WithName(constants.OpenVSXDatabaseComponentName)

func NewOpenVSXDatabaseReconciler() *OpenVSXDatabaseReconciler {
	return &OpenVSXDatabaseReconciler{}
}

func (p *OpenVSXDatabaseReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		p.deleteResources(ctx)
		return reconcile.Result{}, true, nil
	}

	err := p.syncService(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync service: %w", err)
	}

	err = p.syncPVC(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync pvc: %w", err)
	}

	done, err := p.syncDeployment(ctx)
	if !done {
		if err != nil {
			err = fmt.Errorf("failed to sync deployment: %w", err)
		}
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (p *OpenVSXDatabaseReconciler) Finalize(_ *chetypes.DeployContext) bool {
	return true
}

func (p *OpenVSXDatabaseReconciler) deleteResources(ctx *chetypes.DeployContext) {
	objectKey := types.NamespacedName{
		Name:      constants.OpenVSXDatabaseComponentName,
		Namespace: ctx.CheCluster.Namespace,
	}
	cw := ctx.ClusterAPI.ClientWrapper

	err := cw.DeleteByKeyIgnoreNotFound(context.TODO(), objectKey, &appsv1.Deployment{})
	if err != nil {
		logger.Error(err, "Failed to delete Deployment", "Name", ctx.CheCluster.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objectKey, &corev1.Service{})
	if err != nil {
		logger.Error(err, "Failed to delete Service", "Name", ctx.CheCluster.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objectKey, &corev1.PersistentVolumeClaim{})
	if err != nil {
		logger.Error(err, "Failed to delete PVC", "Name", ctx.CheCluster.Name)
	}
}
