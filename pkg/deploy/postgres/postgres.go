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
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
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
	reconciler.Reconcilable
}

func NewPostgresReconciler() *PostgresReconciler {
	return &PostgresReconciler{}
}

func (p *PostgresReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// PostgreSQL component is not used anymore
	_, _ = deploy.DeleteNamespacedObject(ctx, postgresComponentName, &appsv1.Deployment{})
	_, _ = deploy.DeleteNamespacedObject(ctx, backupServerComponentName, &appsv1.Deployment{})
	_, _ = deploy.DeleteNamespacedObject(ctx, defaultPostgresVolumeClaimName, &corev1.PersistentVolumeClaim{})
	_, _ = deploy.DeleteNamespacedObject(ctx, defaultPostgresCredentialsSecret, &corev1.Secret{})
	_, _ = deploy.DeleteNamespacedObject(ctx, postgresComponentName, &corev1.Service{})

	return reconcile.Result{}, true, nil
}

func (p *PostgresReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
