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

package server

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DefaultValuesReconciler struct {
	deploy.Reconcilable
}

func NewDefaultValuesReconciler() *DefaultValuesReconciler {
	return &DefaultValuesReconciler{}
}

func (p *DefaultValuesReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	cheNamespace := ctx.CheCluster.Namespace

	if len(ctx.CheCluster.Spec.Database.ChePostgresSecret) < 1 {
		if len(ctx.CheCluster.Spec.Database.ChePostgresUser) < 1 || len(ctx.CheCluster.Spec.Database.ChePostgresPassword) < 1 {
			chePostgresSecret := deploy.DefaultChePostgresSecret()
			_, err := deploy.SyncSecretToCluster(ctx, chePostgresSecret, cheNamespace, map[string][]byte{"user": []byte(deploy.DefaultChePostgresUser), "password": []byte(util.GeneratePasswd(12))})
			if err != nil {
				return reconcile.Result{}, false, err
			}
			ctx.CheCluster.Spec.Database.ChePostgresSecret = chePostgresSecret
			if err := deploy.UpdateCheCRSpec(ctx, "Postgres Secret", chePostgresSecret); err != nil {
				return reconcile.Result{}, false, err
			}
		} else {
			if len(ctx.CheCluster.Spec.Database.ChePostgresUser) < 1 {
				ctx.CheCluster.Spec.Database.ChePostgresUser = deploy.DefaultChePostgresUser
				if err := deploy.UpdateCheCRSpec(ctx, "Postgres User", ctx.CheCluster.Spec.Database.ChePostgresUser); err != nil {
					return reconcile.Result{}, false, err
				}
			}
			if len(ctx.CheCluster.Spec.Database.ChePostgresPassword) < 1 {
				ctx.CheCluster.Spec.Database.ChePostgresPassword = util.GeneratePasswd(12)
				if err := deploy.UpdateCheCRSpec(ctx, "auto-generated CheCluster DB password", "password-hidden"); err != nil {
					return reconcile.Result{}, false, err
				}
			}
		}
	}

	chePostgresDb := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresDb, "dbche")
	if len(ctx.CheCluster.Spec.Database.ChePostgresDb) < 1 {
		ctx.CheCluster.Spec.Database.ChePostgresDb = chePostgresDb
		if err := deploy.UpdateCheCRSpec(ctx, "Postgres DB", chePostgresDb); err != nil {
			return reconcile.Result{}, false, err
		}
	}
	chePostgresHostName := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
	if len(ctx.CheCluster.Spec.Database.ChePostgresHostName) < 1 {
		ctx.CheCluster.Spec.Database.ChePostgresHostName = chePostgresHostName
		if err := deploy.UpdateCheCRSpec(ctx, "Postgres hostname", chePostgresHostName); err != nil {
			return reconcile.Result{}, false, err
		}
	}
	chePostgresPort := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
	if len(ctx.CheCluster.Spec.Database.ChePostgresPort) < 1 {
		ctx.CheCluster.Spec.Database.ChePostgresPort = chePostgresPort
		if err := deploy.UpdateCheCRSpec(ctx, "Postgres port", chePostgresPort); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	cheLogLevel := util.GetValue(ctx.CheCluster.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	if len(ctx.CheCluster.Spec.Server.CheLogLevel) < 1 {
		ctx.CheCluster.Spec.Server.CheLogLevel = cheLogLevel
		if err := deploy.UpdateCheCRSpec(ctx, "log level", cheLogLevel); err != nil {
			return reconcile.Result{}, false, err
		}
	}
	cheDebug := util.GetValue(ctx.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	if len(ctx.CheCluster.Spec.Server.CheDebug) < 1 {
		ctx.CheCluster.Spec.Server.CheDebug = cheDebug
		if err := deploy.UpdateCheCRSpec(ctx, "debug", cheDebug); err != nil {
			return reconcile.Result{}, false, err
		}
	}
	pvcStrategy := util.GetValue(ctx.CheCluster.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	if len(ctx.CheCluster.Spec.Storage.PvcStrategy) < 1 {
		ctx.CheCluster.Spec.Storage.PvcStrategy = pvcStrategy
		if err := deploy.UpdateCheCRSpec(ctx, "pvc strategy", pvcStrategy); err != nil {
			return reconcile.Result{}, false, err
		}
	}
	pvcClaimSize := util.GetValue(ctx.CheCluster.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	if len(ctx.CheCluster.Spec.Storage.PvcClaimSize) < 1 {
		ctx.CheCluster.Spec.Storage.PvcClaimSize = pvcClaimSize
		if err := deploy.UpdateCheCRSpec(ctx, "pvc claim size", pvcClaimSize); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (p *DefaultValuesReconciler) Finalize(ctx *deploy.DeployContext) bool {
	return true
}
