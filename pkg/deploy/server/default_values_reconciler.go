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
	"strconv"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DefaultValuesReconciler struct {
	deploy.Reconcilable
}

func NewDefaultValuesReconciler() *DefaultValuesReconciler {
	return &DefaultValuesReconciler{}
}

func (p *DefaultValuesReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	cheFlavor := deploy.DefaultCheFlavor(ctx.CheCluster)
	cheNamespace := ctx.CheCluster.Namespace
	if len(ctx.CheCluster.Spec.Server.CheFlavor) < 1 {
		ctx.CheCluster.Spec.Server.CheFlavor = cheFlavor
		if err := deploy.UpdateCheCRSpec(ctx, "installation flavor", cheFlavor); err != nil {
			return reconcile.Result{}, false, err
		}
	}

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
	if len(ctx.CheCluster.Spec.Auth.IdentityProviderPostgresSecret) < 1 {
		keycloakPostgresPassword := util.GeneratePasswd(12)
		keycloakDeployment := &appsv1.Deployment{}
		exists, err := deploy.GetNamespacedObject(ctx, deploy.IdentityProviderName, keycloakDeployment)
		if err != nil {
			logrus.Error(err)
		}
		if exists {
			keycloakPostgresPassword = util.GetDeploymentEnv(keycloakDeployment, "DB_PASSWORD")
		}

		if len(ctx.CheCluster.Spec.Auth.IdentityProviderPostgresPassword) < 1 {
			identityPostgresSecret := deploy.DefaultCheIdentityPostgresSecret()
			_, err := deploy.SyncSecretToCluster(ctx, identityPostgresSecret, cheNamespace, map[string][]byte{"password": []byte(keycloakPostgresPassword)})
			if err != nil {
				return reconcile.Result{}, false, err
			}
			ctx.CheCluster.Spec.Auth.IdentityProviderPostgresSecret = identityPostgresSecret
			if err := deploy.UpdateCheCRSpec(ctx, "Identity Provider Postgres Secret", identityPostgresSecret); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	}

	if len(ctx.CheCluster.Spec.Auth.IdentityProviderSecret) < 1 {
		keycloakAdminUserName := util.GetValue(ctx.CheCluster.Spec.Auth.IdentityProviderAdminUserName, "admin")
		keycloakAdminPassword := util.GetValue(ctx.CheCluster.Spec.Auth.IdentityProviderPassword, util.GeneratePasswd(12))

		keycloakDeployment := &appsv1.Deployment{}
		exists, _ := deploy.GetNamespacedObject(ctx, deploy.IdentityProviderName, keycloakDeployment)
		if exists {
			keycloakAdminUserName = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_USERNAME")
			keycloakAdminPassword = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_PASSWORD")
		}

		if len(ctx.CheCluster.Spec.Auth.IdentityProviderAdminUserName) < 1 || len(ctx.CheCluster.Spec.Auth.IdentityProviderPassword) < 1 {
			identityProviderSecret := deploy.DefaultCheIdentitySecret()
			_, err := deploy.SyncSecretToCluster(ctx, identityProviderSecret, cheNamespace, map[string][]byte{"user": []byte(keycloakAdminUserName), "password": []byte(keycloakAdminPassword)})
			if err != nil {
				return reconcile.Result{}, false, err
			}
			ctx.CheCluster.Spec.Auth.IdentityProviderSecret = identityProviderSecret
			if err := deploy.UpdateCheCRSpec(ctx, "Identity Provider Secret", identityProviderSecret); err != nil {
				return reconcile.Result{}, false, err
			}
		} else {
			if len(ctx.CheCluster.Spec.Auth.IdentityProviderPassword) < 1 {
				ctx.CheCluster.Spec.Auth.IdentityProviderPassword = keycloakAdminPassword
				if err := deploy.UpdateCheCRSpec(ctx, "Keycloak admin password", "password hidden"); err != nil {
					return reconcile.Result{}, false, err
				}
			}
			if len(ctx.CheCluster.Spec.Auth.IdentityProviderAdminUserName) < 1 {
				ctx.CheCluster.Spec.Auth.IdentityProviderAdminUserName = keycloakAdminUserName
				if err := deploy.UpdateCheCRSpec(ctx, "Keycloak admin username", keycloakAdminUserName); err != nil {
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

	if !ctx.CheCluster.IsNativeUserModeEnabled() {
		keycloakRealm := util.GetValue(ctx.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
		if len(ctx.CheCluster.Spec.Auth.IdentityProviderRealm) < 1 {
			ctx.CheCluster.Spec.Auth.IdentityProviderRealm = keycloakRealm
			if err := deploy.UpdateCheCRSpec(ctx, "Keycloak realm", keycloakRealm); err != nil {
				return reconcile.Result{}, false, err
			}
		}
		keycloakClientId := util.GetValue(ctx.CheCluster.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
		if len(ctx.CheCluster.Spec.Auth.IdentityProviderClientId) < 1 {
			ctx.CheCluster.Spec.Auth.IdentityProviderClientId = keycloakClientId

			if err := deploy.UpdateCheCRSpec(ctx, "Keycloak client ID", keycloakClientId); err != nil {
				return reconcile.Result{}, false, err
			}
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

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next
	// version that should fixed bug https://github.com/eclipse/che/issues/13714
	// Or for the transition from CRW 1.2 to 2.0

	if ctx.CheCluster.Spec.Storage.PvcJobsImage == deploy.OldDefaultPvcJobsUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(ctx.CheCluster) && ctx.CheCluster.Spec.Storage.PvcJobsImage != "") {
		ctx.CheCluster.Spec.Storage.PvcJobsImage = ""
		if err := deploy.UpdateCheCRSpec(ctx, "pvc jobs image", ctx.CheCluster.Spec.Storage.PvcJobsImage); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if ctx.CheCluster.Spec.Database.PostgresImage == deploy.OldDefaultPostgresUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(ctx.CheCluster) && ctx.CheCluster.Spec.Database.PostgresImage != "") {
		ctx.CheCluster.Spec.Database.PostgresImage = ""
		if err := deploy.UpdateCheCRSpec(ctx, "postgres image", ctx.CheCluster.Spec.Database.PostgresImage); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if ctx.CheCluster.Spec.Auth.IdentityProviderImage == deploy.OldDefaultKeycloakUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(ctx.CheCluster) && ctx.CheCluster.Spec.Auth.IdentityProviderImage != "") {
		ctx.CheCluster.Spec.Auth.IdentityProviderImage = ""
		if err := deploy.UpdateCheCRSpec(ctx, "keycloak image", ctx.CheCluster.Spec.Auth.IdentityProviderImage); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if deploy.MigratingToCRW2_0(ctx.CheCluster) &&
		!ctx.CheCluster.Spec.Server.ExternalPluginRegistry &&
		ctx.CheCluster.Spec.Server.PluginRegistryUrl == deploy.OldCrwPluginRegistryUrl {
		ctx.CheCluster.Spec.Server.PluginRegistryUrl = ""
		if err := deploy.UpdateCheCRSpec(ctx, "plugin registry url", ctx.CheCluster.Spec.Server.PluginRegistryUrl); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if deploy.MigratingToCRW2_0(ctx.CheCluster) &&
		ctx.CheCluster.Spec.Server.CheImage == deploy.OldDefaultCodeReadyServerImageRepo {
		ctx.CheCluster.Spec.Server.CheImage = ""
		if err := deploy.UpdateCheCRSpec(ctx, "che image repo", ctx.CheCluster.Spec.Server.CheImage); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if deploy.MigratingToCRW2_0(ctx.CheCluster) &&
		ctx.CheCluster.Spec.Server.CheImageTag == deploy.OldDefaultCodeReadyServerImageTag {
		ctx.CheCluster.Spec.Server.CheImageTag = ""
		if err := deploy.UpdateCheCRSpec(ctx, "che image tag", ctx.CheCluster.Spec.Server.CheImageTag); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if ctx.CheCluster.Spec.Server.ServerExposureStrategy == "" && ctx.CheCluster.Spec.K8s.IngressStrategy == "" {
		strategy := util.GetServerExposureStrategy(ctx.CheCluster)
		ctx.CheCluster.Spec.Server.ServerExposureStrategy = strategy
		if err := deploy.UpdateCheCRSpec(ctx, "serverExposureStrategy", strategy); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if ctx.CheCluster.Spec.DevWorkspace.Enable && ctx.CheCluster.Spec.Auth.NativeUserMode == nil {
		newNativeUserModeValue := util.NewBoolPointer(true)
		ctx.CheCluster.Spec.Auth.NativeUserMode = newNativeUserModeValue
		if err := deploy.UpdateCheCRSpec(ctx, "nativeUserMode", strconv.FormatBool(*newNativeUserModeValue)); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (p *DefaultValuesReconciler) Finalize(ctx *deploy.DeployContext) error {
	return nil
}
