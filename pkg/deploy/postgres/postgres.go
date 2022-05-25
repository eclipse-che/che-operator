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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
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

func (p *PostgresReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Components.Database.ExternalDb {
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

	if ctx.CheCluster.Status.PostgresVersion == "" {
		if !test.IsTestMode() { // ignore in tests
			done, err := p.setDbVersion(ctx)
			if !done {
				return reconcile.Result{}, false, err
			}
		}
	}

	return reconcile.Result{}, true, nil
}

func (p *PostgresReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (p *PostgresReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(ctx, constants.PostgresName, []string{constants.PostgresName}, []int32{5432}, constants.PostgresName)
}

func (p *PostgresReconciler) syncPVC(ctx *chetypes.DeployContext) (bool, error) {
	pvc := ctx.CheCluster.Spec.Components.Database.Pvc.DeepCopy()
	pvc.ClaimSize = utils.GetValue(ctx.CheCluster.Spec.Components.Database.Pvc.ClaimSize, constants.DefaultPostgresPvcClaimSize)

	done, err := deploy.SyncPVCToCluster(ctx, constants.DefaultPostgresVolumeClaimName, pvc, constants.PostgresName)
	if !done {
		if err == nil {
			logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", constants.DefaultPostgresVolumeClaimName)
		}
	}
	return done, err
}

func (p *PostgresReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	clusterDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(ctx, constants.PostgresName, clusterDeployment)
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

func (p *PostgresReconciler) setDbVersion(ctx *chetypes.DeployContext) (bool, error) {
	k8sHelper := k8shelper.New()
	postgresVersion, err := k8sHelper.ExecIntoPod(
		constants.PostgresName,
		"postgres -V | awk '{print $NF}' | cut -d '.' -f1-2",
		"get PostgreSQL version",
		ctx.CheCluster.Namespace)
	if err != nil {
		return false, err
	}

	postgresVersion = strings.TrimSpace(postgresVersion)
	ctx.CheCluster.Status.PostgresVersion = postgresVersion
	err = deploy.UpdateCheCRStatus(ctx, "postgresVersion", postgresVersion)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Create secret with PostgreSQL credentials.
func (p *PostgresReconciler) syncCredentials(ctx *chetypes.DeployContext) (bool, error) {
	postgresCredentialsSecretName := utils.GetValue(ctx.CheCluster.Spec.Components.Database.CredentialsSecretName, constants.DefaultPostgresCredentialsSecret)
	exists, err := deploy.GetNamespacedObject(ctx, postgresCredentialsSecretName, &corev1.Secret{})
	if err != nil {
		return false, err
	}

	if !exists {
		postgresUser := constants.DefaultPostgresUser
		postgresPassword := utils.GeneratePassword(12)
		return deploy.SyncSecretToCluster(ctx, postgresCredentialsSecretName, ctx.CheCluster.Namespace, map[string][]byte{"user": []byte(postgresUser), "password": []byte(postgresPassword)})
	}

	return true, nil
}
