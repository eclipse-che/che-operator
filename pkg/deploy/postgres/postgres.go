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
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PostgresReconciler struct {
	deploy.Reconcilable
}

func NewPostgresReconciler() *PostgresReconciler {
	return &PostgresReconciler{}
}

func (p *PostgresReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Database.ExternalDb {
		return reconcile.Result{}, true, nil
	}

	done, err := p.syncCredentials(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncPVC(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	if ctx.CheCluster.Spec.Database.PostgresVersion == "" {
		if !util.IsTestMode() { // ignore in tests
			done, err := p.setDbVersion(ctx)
			if !done {
				return reconcile.Result{}, false, err
			}
		}
	}

	return reconcile.Result{}, true, nil
}

func (p *PostgresReconciler) Finalize(ctx *deploy.DeployContext) bool {
	return true
}

func (p *PostgresReconciler) syncService(ctx *deploy.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(ctx, deploy.PostgresName, []string{deploy.PostgresName}, []int32{5432}, deploy.PostgresName)
}

func (p *PostgresReconciler) syncPVC(ctx *deploy.DeployContext) (bool, error) {
	pvcClaimSize := util.GetValue(ctx.CheCluster.Spec.Database.PvcClaimSize, deploy.DefaultPostgresPvcClaimSize)
	done, err := deploy.SyncPVCToCluster(ctx, deploy.DefaultPostgresVolumeClaimName, pvcClaimSize, deploy.PostgresName)
	if !done {
		if err == nil {
			logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", deploy.DefaultPostgresVolumeClaimName)
		}
	}
	return done, err
}

func (p *PostgresReconciler) syncDeployment(ctx *deploy.DeployContext) (bool, error) {
	clusterDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(ctx, deploy.PostgresName, clusterDeployment)
	if err != nil {
		return false, err
	}

	if !exists {
		clusterDeployment = nil
	}

	specDeployment, err := p.getDeploymentSpec(clusterDeployment, ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, specDeployment, deploy.DefaultDeploymentDiffOpts)
}

func (p *PostgresReconciler) setDbVersion(ctx *deploy.DeployContext) (bool, error) {
	postgresVersion, err := util.K8sclient.ExecIntoPod(
		ctx.CheCluster,
		deploy.PostgresName,
		func(cr *orgv1.CheCluster) (string, error) {
			// don't take into account bugfix version
			return "postgres -V | awk '{print $NF}' | cut -d '.' -f1-2", nil
		},
		"get PostgreSQL version")
	if err != nil {
		return false, err
	}

	postgresVersion = strings.TrimSpace(postgresVersion)
	ctx.CheCluster.Spec.Database.PostgresVersion = postgresVersion
	err = deploy.UpdateCheCRSpec(ctx, "database.postgresVersion", postgresVersion)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Create secret with PostgreSQL credentials.
func (p *PostgresReconciler) syncCredentials(ctx *deploy.DeployContext) (bool, error) {
	postgresCredentialsSecretRef := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresSecret, deploy.DefaultChePostgresCredentialsSecret)
	exists, err := deploy.GetNamespacedObject(ctx, postgresCredentialsSecretRef, &corev1.Secret{})
	if err != nil {
		return false, err
	}

	if !exists {
		postgresUser := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresUser, deploy.DefaultChePostgresUser)
		postgresPassword := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresPassword, util.GeneratePasswd(12))
		return deploy.SyncSecretToCluster(ctx, postgresCredentialsSecretRef, ctx.CheCluster.Namespace, map[string][]byte{"user": []byte(postgresUser), "password": []byte(postgresPassword)})
	}

	return true, nil
}
