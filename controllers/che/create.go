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

package che

import (
	"strconv"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
)

func (r *CheClusterReconciler) GenerateAndSaveFields(deployContext *deploy.DeployContext) (err error) {
	cheFlavor := deploy.DefaultCheFlavor(deployContext.CheCluster)
	cheNamespace := deployContext.CheCluster.Namespace
	if len(deployContext.CheCluster.Spec.Server.CheFlavor) < 1 {
		deployContext.CheCluster.Spec.Server.CheFlavor = cheFlavor
		if err := deploy.UpdateCheCRSpec(deployContext, "installation flavor", cheFlavor); err != nil {
			return err
		}
	}

	if len(deployContext.CheCluster.Spec.Database.ChePostgresSecret) < 1 {
		if len(deployContext.CheCluster.Spec.Database.ChePostgresUser) < 1 || len(deployContext.CheCluster.Spec.Database.ChePostgresPassword) < 1 {
			chePostgresSecret := deploy.DefaultChePostgresSecret()
			_, err := deploy.SyncSecretToCluster(deployContext, chePostgresSecret, cheNamespace, map[string][]byte{"user": []byte(deploy.DefaultChePostgresUser), "password": []byte(util.GeneratePasswd(12))})
			if err != nil {
				return err
			}
			deployContext.CheCluster.Spec.Database.ChePostgresSecret = chePostgresSecret
			if err := deploy.UpdateCheCRSpec(deployContext, "Postgres Secret", chePostgresSecret); err != nil {
				return err
			}
		} else {
			if len(deployContext.CheCluster.Spec.Database.ChePostgresUser) < 1 {
				deployContext.CheCluster.Spec.Database.ChePostgresUser = deploy.DefaultChePostgresUser
				if err := deploy.UpdateCheCRSpec(deployContext, "Postgres User", deployContext.CheCluster.Spec.Database.ChePostgresUser); err != nil {
					return err
				}
			}
			if len(deployContext.CheCluster.Spec.Database.ChePostgresPassword) < 1 {
				deployContext.CheCluster.Spec.Database.ChePostgresPassword = util.GeneratePasswd(12)
				if err := deploy.UpdateCheCRSpec(deployContext, "auto-generated CheCluster DB password", "password-hidden"); err != nil {
					return err
				}
			}
		}
	}
	if len(deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresSecret) < 1 {
		keycloakPostgresPassword := util.GeneratePasswd(12)
		keycloakDeployment := &appsv1.Deployment{}
		exists, err := deploy.GetNamespacedObject(deployContext, deploy.IdentityProviderName, keycloakDeployment)
		if err != nil {
			logrus.Error(err)
		}
		if exists {
			keycloakPostgresPassword = util.GetDeploymentEnv(keycloakDeployment, "DB_PASSWORD")
		}

		if len(deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresPassword) < 1 {
			identityPostgresSecret := deploy.DefaultCheIdentityPostgresSecret()
			_, err := deploy.SyncSecretToCluster(deployContext, identityPostgresSecret, cheNamespace, map[string][]byte{"password": []byte(keycloakPostgresPassword)})
			if err != nil {
				return err
			}
			deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresSecret = identityPostgresSecret
			if err := deploy.UpdateCheCRSpec(deployContext, "Identity Provider Postgres Secret", identityPostgresSecret); err != nil {
				return err
			}
		}
	}

	if len(deployContext.CheCluster.Spec.Auth.IdentityProviderSecret) < 1 {
		keycloakAdminUserName := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName, "admin")
		keycloakAdminPassword := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderPassword, util.GeneratePasswd(12))

		keycloakDeployment := &appsv1.Deployment{}
		exists, _ := deploy.GetNamespacedObject(deployContext, deploy.IdentityProviderName, keycloakDeployment)
		if exists {
			keycloakAdminUserName = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_USERNAME")
			keycloakAdminPassword = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_PASSWORD")
		}

		if len(deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName) < 1 || len(deployContext.CheCluster.Spec.Auth.IdentityProviderPassword) < 1 {
			identityProviderSecret := deploy.DefaultCheIdentitySecret()
			_, err = deploy.SyncSecretToCluster(deployContext, identityProviderSecret, cheNamespace, map[string][]byte{"user": []byte(keycloakAdminUserName), "password": []byte(keycloakAdminPassword)})
			if err != nil {
				return err
			}
			deployContext.CheCluster.Spec.Auth.IdentityProviderSecret = identityProviderSecret
			if err := deploy.UpdateCheCRSpec(deployContext, "Identity Provider Secret", identityProviderSecret); err != nil {
				return err
			}
		} else {
			if len(deployContext.CheCluster.Spec.Auth.IdentityProviderPassword) < 1 {
				deployContext.CheCluster.Spec.Auth.IdentityProviderPassword = keycloakAdminPassword
				if err := deploy.UpdateCheCRSpec(deployContext, "Keycloak admin password", "password hidden"); err != nil {
					return err
				}
			}
			if len(deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName) < 1 {
				deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName = keycloakAdminUserName
				if err := deploy.UpdateCheCRSpec(deployContext, "Keycloak admin username", keycloakAdminUserName); err != nil {
					return err
				}
			}
		}
	}

	chePostgresDb := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresDb, "dbche")
	if len(deployContext.CheCluster.Spec.Database.ChePostgresDb) < 1 {
		deployContext.CheCluster.Spec.Database.ChePostgresDb = chePostgresDb
		if err := deploy.UpdateCheCRSpec(deployContext, "Postgres DB", chePostgresDb); err != nil {
			return err
		}
	}
	chePostgresHostName := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
	if len(deployContext.CheCluster.Spec.Database.ChePostgresHostName) < 1 {
		deployContext.CheCluster.Spec.Database.ChePostgresHostName = chePostgresHostName
		if err := deploy.UpdateCheCRSpec(deployContext, "Postgres hostname", chePostgresHostName); err != nil {
			return err
		}
	}
	chePostgresPort := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
	if len(deployContext.CheCluster.Spec.Database.ChePostgresPort) < 1 {
		deployContext.CheCluster.Spec.Database.ChePostgresPort = chePostgresPort
		if err := deploy.UpdateCheCRSpec(deployContext, "Postgres port", chePostgresPort); err != nil {
			return err
		}
	}

	if !util.IsOpenShift || !deployContext.CheCluster.IsNativeUserModeEnabled() {
		keycloakRealm := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
		if len(deployContext.CheCluster.Spec.Auth.IdentityProviderRealm) < 1 {
			deployContext.CheCluster.Spec.Auth.IdentityProviderRealm = keycloakRealm
			if err := deploy.UpdateCheCRSpec(deployContext, "Keycloak realm", keycloakRealm); err != nil {
				return err
			}
		}
		keycloakClientId := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
		if len(deployContext.CheCluster.Spec.Auth.IdentityProviderClientId) < 1 {
			deployContext.CheCluster.Spec.Auth.IdentityProviderClientId = keycloakClientId

			if err := deploy.UpdateCheCRSpec(deployContext, "Keycloak client ID", keycloakClientId); err != nil {
				return err
			}
		}
	}

	cheLogLevel := util.GetValue(deployContext.CheCluster.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	if len(deployContext.CheCluster.Spec.Server.CheLogLevel) < 1 {
		deployContext.CheCluster.Spec.Server.CheLogLevel = cheLogLevel
		if err := deploy.UpdateCheCRSpec(deployContext, "log level", cheLogLevel); err != nil {
			return err
		}
	}
	cheDebug := util.GetValue(deployContext.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	if len(deployContext.CheCluster.Spec.Server.CheDebug) < 1 {
		deployContext.CheCluster.Spec.Server.CheDebug = cheDebug
		if err := deploy.UpdateCheCRSpec(deployContext, "debug", cheDebug); err != nil {
			return err
		}
	}
	pvcStrategy := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	if len(deployContext.CheCluster.Spec.Storage.PvcStrategy) < 1 {
		deployContext.CheCluster.Spec.Storage.PvcStrategy = pvcStrategy
		if err := deploy.UpdateCheCRSpec(deployContext, "pvc strategy", pvcStrategy); err != nil {
			return err
		}
	}
	pvcClaimSize := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	if len(deployContext.CheCluster.Spec.Storage.PvcClaimSize) < 1 {
		deployContext.CheCluster.Spec.Storage.PvcClaimSize = pvcClaimSize
		if err := deploy.UpdateCheCRSpec(deployContext, "pvc claim size", pvcClaimSize); err != nil {
			return err
		}
	}

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next
	// version that should fixed bug https://github.com/eclipse/che/issues/13714
	// Or for the transition from CRW 1.2 to 2.0

	if deployContext.CheCluster.Spec.Storage.PvcJobsImage == deploy.OldDefaultPvcJobsUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(deployContext.CheCluster) && deployContext.CheCluster.Spec.Storage.PvcJobsImage != "") {
		deployContext.CheCluster.Spec.Storage.PvcJobsImage = ""
		if err := deploy.UpdateCheCRSpec(deployContext, "pvc jobs image", deployContext.CheCluster.Spec.Storage.PvcJobsImage); err != nil {
			return err
		}
	}

	if deployContext.CheCluster.Spec.Database.PostgresImage == deploy.OldDefaultPostgresUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(deployContext.CheCluster) && deployContext.CheCluster.Spec.Database.PostgresImage != "") {
		deployContext.CheCluster.Spec.Database.PostgresImage = ""
		if err := deploy.UpdateCheCRSpec(deployContext, "postgres image", deployContext.CheCluster.Spec.Database.PostgresImage); err != nil {
			return err
		}
	}

	if deployContext.CheCluster.Spec.Auth.IdentityProviderImage == deploy.OldDefaultKeycloakUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(deployContext.CheCluster) && deployContext.CheCluster.Spec.Auth.IdentityProviderImage != "") {
		deployContext.CheCluster.Spec.Auth.IdentityProviderImage = ""
		if err := deploy.UpdateCheCRSpec(deployContext, "keycloak image", deployContext.CheCluster.Spec.Auth.IdentityProviderImage); err != nil {
			return err
		}
	}

	if deploy.MigratingToCRW2_0(deployContext.CheCluster) &&
		!deployContext.CheCluster.Spec.Server.ExternalPluginRegistry &&
		deployContext.CheCluster.Spec.Server.PluginRegistryUrl == deploy.OldCrwPluginRegistryUrl {
		deployContext.CheCluster.Spec.Server.PluginRegistryUrl = ""
		if err := deploy.UpdateCheCRSpec(deployContext, "plugin registry url", deployContext.CheCluster.Spec.Server.PluginRegistryUrl); err != nil {
			return err
		}
	}

	if deploy.MigratingToCRW2_0(deployContext.CheCluster) &&
		deployContext.CheCluster.Spec.Server.CheImage == deploy.OldDefaultCodeReadyServerImageRepo {
		deployContext.CheCluster.Spec.Server.CheImage = ""
		if err := deploy.UpdateCheCRSpec(deployContext, "che image repo", deployContext.CheCluster.Spec.Server.CheImage); err != nil {
			return err
		}
	}

	if deploy.MigratingToCRW2_0(deployContext.CheCluster) &&
		deployContext.CheCluster.Spec.Server.CheImageTag == deploy.OldDefaultCodeReadyServerImageTag {
		deployContext.CheCluster.Spec.Server.CheImageTag = ""
		if err := deploy.UpdateCheCRSpec(deployContext, "che image tag", deployContext.CheCluster.Spec.Server.CheImageTag); err != nil {
			return err
		}
	}

	if deployContext.CheCluster.Spec.Server.ServerExposureStrategy == "" && deployContext.CheCluster.Spec.K8s.IngressStrategy == "" {
		strategy := util.GetServerExposureStrategy(deployContext.CheCluster)
		deployContext.CheCluster.Spec.Server.ServerExposureStrategy = strategy
		if err := deploy.UpdateCheCRSpec(deployContext, "serverExposureStrategy", strategy); err != nil {
			return err
		}
	}

	if util.IsOpenShift && deployContext.CheCluster.Spec.DevWorkspace.Enable && deployContext.CheCluster.Spec.Auth.NativeUserMode == nil {
		newNativeUserModeValue := util.NewBoolPointer(true)
		deployContext.CheCluster.Spec.Auth.NativeUserMode = newNativeUserModeValue
		if err := deploy.UpdateCheCRSpec(deployContext, "nativeUserMode", strconv.FormatBool(*newNativeUserModeValue)); err != nil {
			return err
		}
	}

	return nil
}
