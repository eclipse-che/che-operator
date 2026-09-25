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

package postgres

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	objects := []struct {
		name string
		obj  client.Object
	}{
		{postgresComponentName, &appsv1.Deployment{}},
		{backupServerComponentName, &appsv1.Deployment{}},
		{defaultPostgresVolumeClaimName, &corev1.PersistentVolumeClaim{}},
		{defaultPostgresCredentialsSecret, &corev1.Secret{}},
		{postgresComponentName, &corev1.Service{}},
	}

	for _, object := range objects {
		key := types.NamespacedName{Name: object.name, Namespace: ctx.CheCluster.Namespace}
		_ = ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(ctx.Context, key, object.obj)
	}

	return reconcile.Result{}, true, nil
}

func (p *PostgresReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
