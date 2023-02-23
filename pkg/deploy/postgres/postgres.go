//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package postgres

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PostgresReconciler struct {
	deploy.Reconcilable
}

func NewPostgresReconciler() *PostgresReconciler {
	return &PostgresReconciler{}
}

func (p *PostgresReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// PostgreSQL is deprecated and will be removed in the future
	_, _ = p.syncPostgreDeployment(ctx)
	_, _ = p.syncDbVersion(ctx)

	// Backup server component has been already removed
	_, _ = p.syncBackupDeployment(ctx)

	return reconcile.Result{}, true, nil
}

func (p *PostgresReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (p *PostgresReconciler) syncPostgreDeployment(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, constants.PostgresName, &appsv1.Deployment{})
}

func (p *PostgresReconciler) syncBackupDeployment(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, constants.BackupServerComponentName, &appsv1.Deployment{})
}

func (p *PostgresReconciler) syncDbVersion(ctx *chetypes.DeployContext) (bool, error) {
	if ctx.CheCluster.Status.PostgresVersion != "" {
		ctx.CheCluster.Status.PostgresVersion = ""
		_ = deploy.UpdateCheCRStatus(ctx, "postgresVersion", ctx.CheCluster.Status.PostgresVersion)
	}
	return true, nil
}
