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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXDatabaseReconciler struct {
	reconciler.Reconcilable

	// databaseProvisioned prevents recreating the setup Job on every reconcile.
	// Resets on operator restart, which is safe - the Job uses ON CONFLICT DO NOTHING.
	databaseProvisioned bool
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

	if !p.databaseProvisioned {
		err = p.syncDatabaseProvisioned(ctx)
		if err != nil {
			return reconcile.Result{}, false, fmt.Errorf("failed to provision database: %w", err)
		}
	}

	return reconcile.Result{}, true, nil
}

func (p *OpenVSXDatabaseReconciler) Finalize(_ *chetypes.DeployContext) bool {
	return true
}

func (p *OpenVSXDatabaseReconciler) deleteResources(ctx *chetypes.DeployContext) {
	objtKey := types.NamespacedName{
		Name:      constants.OpenVSXDatabaseComponentName,
		Namespace: ctx.CheCluster.Namespace,
	}
	cw := ctx.ClusterAPI.ClientWrapper

	err := cw.DeleteByKeyIgnoreNotFound(context.TODO(), objtKey, &appsv1.Deployment{})
	if err != nil {
		logger.Error(err, "Failed to delete Deployment", "Name", objtKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objtKey, &corev1.Service{})
	if err != nil {
		logger.Error(err, "Failed to delete Service", "Name", objtKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objtKey, &corev1.PersistentVolumeClaim{})
	if err != nil {
		logger.Error(err, "Failed to delete PVC", "Name", objtKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXDatabaseProvisionJobName,
			Namespace: ctx.CheCluster.Namespace,
		},
		&batchv1.Job{},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)
	if err != nil {
		logger.Error(err, "Failed to delete Job", "Name", constants.OpenVSXDatabaseProvisionJobName)
	}
}
