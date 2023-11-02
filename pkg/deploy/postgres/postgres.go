//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultPostgresCredentialsSecret = "postgres-credentials"
	defaultPostgresVolumeClaimName   = "postgres-data"
	postgresComponentName            = "postgres"
	backupServerComponentName        = "backup-rest-server-deployment"
)

type PostgresReconciler struct {
	deploy.Reconcilable
}

func NewPostgresReconciler() *PostgresReconciler {
	return &PostgresReconciler{}
}

func (p *PostgresReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// PostgreSQL component is not used anymore
	_, _ = p.syncDeployment(ctx)
	_, _ = p.syncPVC(ctx)
	_, _ = p.syncCredentials(ctx)
	_, _ = p.syncService(ctx)

	// Backup server component is not used anymore
	_, _ = p.syncBackupDeployment(ctx)

	return reconcile.Result{}, true, nil
}

func (p *PostgresReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (p *PostgresReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, postgresComponentName, &corev1.Service{})
}

func (p *PostgresReconciler) syncPVC(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, defaultPostgresVolumeClaimName, &corev1.PersistentVolumeClaim{})
}

func (p *PostgresReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, postgresComponentName, &appsv1.Deployment{})
}

func (p *PostgresReconciler) syncCredentials(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, defaultPostgresCredentialsSecret, &corev1.Secret{})
}

func (p *PostgresReconciler) syncBackupDeployment(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.DeleteNamespacedObject(ctx, backupServerComponentName, &appsv1.Deployment{})
}
