//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"context"
	"strings"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileChe) CreateNewOauthClient(instance *orgv1.CheCluster, oAuthClient *oauth.OAuthClient) error {
	oAuthClientFound := &oauth.OAuthClient{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: oAuthClient.Name, Namespace: oAuthClient.Namespace}, oAuthClientFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", oAuthClient.Kind, oAuthClient.Name)
		err = r.client.Create(context.TODO(), oAuthClient)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", oAuthClient.Kind, oAuthClient.Name, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred: %s", err)

		return err
	}
	return nil
}

func (r *ReconcileChe) CreateIdentityProviderItems(instance *orgv1.CheCluster, request reconcile.Request, cheFlavor string, keycloakDeploymentName string, isOpenShift4 bool) (err error) {
	tests := r.tests
	oAuthClientName := instance.Spec.Auth.OAuthClientName
	if len(oAuthClientName) < 1 {
		oAuthClientName = instance.Name + "-openshift-identity-provider-" + strings.ToLower(util.GeneratePasswd(6))
		instance.Spec.Auth.OAuthClientName = oAuthClientName
		if err := r.UpdateCheCRSpec(instance, "oAuthClient name", oAuthClientName); err != nil {
			return err
		}
	}
	oauthSecret := instance.Spec.Auth.OAuthSecret
	if len(oauthSecret) < 1 {
		oauthSecret = util.GeneratePasswd(12)
		instance.Spec.Auth.OAuthSecret = oauthSecret
		if err := r.UpdateCheCRSpec(instance, "oAuthC secret name", oauthSecret); err != nil {
			return err
		}
	}

	keycloakURL := instance.Spec.Auth.IdentityProviderURL
	keycloakRealm := util.GetValue(instance.Spec.Auth.IdentityProviderRealm, cheFlavor)
	oAuthClient := deploy.NewOAuthClient(oAuthClientName, oauthSecret, keycloakURL, keycloakRealm, isOpenShift4)
	if err := r.CreateNewOauthClient(instance, oAuthClient); err != nil {
		return err
	}

	if !tests {
		openShiftIdentityProviderCommand, err := deploy.GetOpenShiftIdentityProviderProvisionCommand(instance, oAuthClientName, oauthSecret, isOpenShift4)
		if err != nil {
			logrus.Errorf("Failed to build identity provider provisioning command")
			return err
		}
		podToExec, err := util.K8sclient.GetDeploymentPod(keycloakDeploymentName, instance.Namespace)
		if err != nil {
			logrus.Errorf("Failed to retrieve pod name. Further exec will fail")
			return err
		}
		_, err = util.K8sclient.ExecIntoPod(podToExec, openShiftIdentityProviderCommand, "create OpenShift identity provider", instance.Namespace)
		if err == nil {
			for {
				instance.Status.OpenShiftoAuthProvisioned = true
				if err := r.UpdateCheCRStatus(instance, "status: provisioned with OpenShift identity provider", "true"); err != nil &&
					errors.IsConflict(err) {
					instance, _ = r.GetCR(request)
					continue
				}
				break
			}
		}
		return err
	}
	return nil
}

func (r *ReconcileChe) GenerateAndSaveFields(instance *orgv1.CheCluster, request reconcile.Request, clusterAPI deploy.ClusterAPI) (err error) {
	cheFlavor := deploy.DefaultCheFlavor(instance)
	if len(instance.Spec.Server.CheFlavor) < 1 {
		instance.Spec.Server.CheFlavor = cheFlavor
		if err := r.UpdateCheCRSpec(instance, "installation flavor", cheFlavor); err != nil {
			return err
		}
	}

	cheMultiUser := deploy.GetCheMultiUser(instance)
	if cheMultiUser == "true" {
		if len(instance.Spec.Database.ChePostgresSecret) < 1 {
			if len(instance.Spec.Database.ChePostgresUser) < 1 || len(instance.Spec.Database.ChePostgresPassword) < 1 {
				chePostgresSecret := deploy.DefaultChePostgresSecret()
				deploy.SyncSecretToCluster(instance, chePostgresSecret, map[string][]byte{"user": []byte(deploy.DefaultChePostgresUser), "password": []byte(util.GeneratePasswd(12))}, clusterAPI)
				instance.Spec.Database.ChePostgresSecret = chePostgresSecret
				if err := r.UpdateCheCRSpec(instance, "Postgres Secret", chePostgresSecret); err != nil {
					return err
				}
			} else {
				if len(instance.Spec.Database.ChePostgresUser) < 1 {
					instance.Spec.Database.ChePostgresUser = deploy.DefaultChePostgresUser
					if err := r.UpdateCheCRSpec(instance, "Postgres User", instance.Spec.Database.ChePostgresUser); err != nil {
						return err
					}
				}
				if len(instance.Spec.Database.ChePostgresPassword) < 1 {
					instance.Spec.Database.ChePostgresPassword = util.GeneratePasswd(12)
					if err := r.UpdateCheCRSpec(instance, "auto-generated CheCluster DB password", "password-hidden"); err != nil {
						return err
					}
				}
			}
		}
		if len(instance.Spec.Auth.IdentityProviderPostgresSecret) < 1 {
			keycloakPostgresPassword := util.GeneratePasswd(12)
			keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
			if err != nil {
				logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating passwd")
			} else {
				keycloakPostgresPassword = util.GetDeploymentEnv(keycloakDeployment, "DB_PASSWORD")
			}

			if len(instance.Spec.Auth.IdentityProviderPostgresPassword) < 1 {
				identityPostgresSecret := deploy.DefaultCheIdentityPostgresSecret()
				deploy.SyncSecretToCluster(instance, identityPostgresSecret, map[string][]byte{"password": []byte(keycloakPostgresPassword)}, clusterAPI)
				instance.Spec.Auth.IdentityProviderPostgresSecret = identityPostgresSecret
				if err := r.UpdateCheCRSpec(instance, "Identity Provider Postgres Secret", identityPostgresSecret); err != nil {
					return err
				}
			}
		}

		if len(instance.Spec.Auth.IdentityProviderSecret) < 1 {
			keycloakAdminUserName := util.GetValue(instance.Spec.Auth.IdentityProviderAdminUserName, "admin")
			keycloakAdminPassword := util.GetValue(instance.Spec.Auth.IdentityProviderPassword, util.GeneratePasswd(12))

			keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
			if err != nil {
				logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating admin username and password")
			} else {
				keycloakAdminUserName = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_USERNAME")
				keycloakAdminPassword = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_PASSWORD")
			}

			if len(instance.Spec.Auth.IdentityProviderAdminUserName) < 1 || len(instance.Spec.Auth.IdentityProviderPassword) < 1 {
				identityProviderSecret := deploy.DefaultCheIdentitySecret()
				deploy.SyncSecretToCluster(instance, identityProviderSecret, map[string][]byte{"user": []byte(keycloakAdminUserName), "password": []byte(keycloakAdminPassword)}, clusterAPI)
				instance.Spec.Auth.IdentityProviderSecret = identityProviderSecret
				if err := r.UpdateCheCRSpec(instance, "Identity Provider Secret", identityProviderSecret); err != nil {
					return err
				}
			} else {
				if len(instance.Spec.Auth.IdentityProviderPassword) < 1 {
					instance.Spec.Auth.IdentityProviderPassword = keycloakAdminPassword
					if err := r.UpdateCheCRSpec(instance, "Keycloak admin password", "password hidden"); err != nil {
						return err
					}
				}
				if len(instance.Spec.Auth.IdentityProviderAdminUserName) < 1 {
					instance.Spec.Auth.IdentityProviderAdminUserName = keycloakAdminUserName
					if err := r.UpdateCheCRSpec(instance, "Keycloak admin username", keycloakAdminUserName); err != nil {
						return err
					}
				}
			}
		}

		chePostgresDb := util.GetValue(instance.Spec.Database.ChePostgresDb, "dbche")
		if len(instance.Spec.Database.ChePostgresDb) < 1 {
			instance.Spec.Database.ChePostgresDb = chePostgresDb
			if err := r.UpdateCheCRSpec(instance, "Postgres DB", chePostgresDb); err != nil {
				return err
			}
		}
		chePostgresHostName := util.GetValue(instance.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
		if len(instance.Spec.Database.ChePostgresHostName) < 1 {
			instance.Spec.Database.ChePostgresHostName = chePostgresHostName
			if err := r.UpdateCheCRSpec(instance, "Postgres hostname", chePostgresHostName); err != nil {
				return err
			}
		}
		chePostgresPort := util.GetValue(instance.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
		if len(instance.Spec.Database.ChePostgresPort) < 1 {
			instance.Spec.Database.ChePostgresPort = chePostgresPort
			if err := r.UpdateCheCRSpec(instance, "Postgres port", chePostgresPort); err != nil {
				return err
			}
		}
		keycloakRealm := util.GetValue(instance.Spec.Auth.IdentityProviderRealm, cheFlavor)
		if len(instance.Spec.Auth.IdentityProviderRealm) < 1 {
			instance.Spec.Auth.IdentityProviderRealm = keycloakRealm
			if err := r.UpdateCheCRSpec(instance, "Keycloak realm", keycloakRealm); err != nil {
				return err
			}
		}
		keycloakClientId := util.GetValue(instance.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
		if len(instance.Spec.Auth.IdentityProviderClientId) < 1 {
			instance.Spec.Auth.IdentityProviderClientId = keycloakClientId

			if err := r.UpdateCheCRSpec(instance, "Keycloak client ID", keycloakClientId); err != nil {
				return err
			}
		}
	}

	cheLogLevel := util.GetValue(instance.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	if len(instance.Spec.Server.CheLogLevel) < 1 {
		instance.Spec.Server.CheLogLevel = cheLogLevel
		if err := r.UpdateCheCRSpec(instance, "log level", cheLogLevel); err != nil {
			return err
		}
	}
	cheDebug := util.GetValue(instance.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	if len(instance.Spec.Server.CheDebug) < 1 {
		instance.Spec.Server.CheDebug = cheDebug
		if err := r.UpdateCheCRSpec(instance, "debug", cheDebug); err != nil {
			return err
		}
	}
	pvcStrategy := util.GetValue(instance.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	if len(instance.Spec.Storage.PvcStrategy) < 1 {
		instance.Spec.Storage.PvcStrategy = pvcStrategy
		if err := r.UpdateCheCRSpec(instance, "pvc strategy", pvcStrategy); err != nil {
			return err
		}
	}
	pvcClaimSize := util.GetValue(instance.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	if len(instance.Spec.Storage.PvcClaimSize) < 1 {
		instance.Spec.Storage.PvcClaimSize = pvcClaimSize
		if err := r.UpdateCheCRSpec(instance, "pvc claim size", pvcClaimSize); err != nil {
			return err
		}
	}

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next
	// version that should fixed bug https://github.com/eclipse/che/issues/13714
	// Or for the transition from CRW 1.2 to 2.0

	if instance.Spec.Storage.PvcJobsImage == deploy.OldDefaultPvcJobsUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(instance) && instance.Spec.Storage.PvcJobsImage != "") {
		instance.Spec.Storage.PvcJobsImage = ""
		if err := r.UpdateCheCRSpec(instance, "pvc jobs image", instance.Spec.Storage.PvcJobsImage); err != nil {
			return err
		}
	}

	if instance.Spec.Database.PostgresImage == deploy.OldDefaultPostgresUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(instance) && instance.Spec.Database.PostgresImage != "") {
		instance.Spec.Database.PostgresImage = ""
		if err := r.UpdateCheCRSpec(instance, "postgres image", instance.Spec.Database.PostgresImage); err != nil {
			return err
		}
	}

	if instance.Spec.Auth.IdentityProviderImage == deploy.OldDefaultKeycloakUpstreamImageToDetect ||
		(deploy.MigratingToCRW2_0(instance) && instance.Spec.Auth.IdentityProviderImage != "") {
		instance.Spec.Auth.IdentityProviderImage = ""
		if err := r.UpdateCheCRSpec(instance, "keycloak image", instance.Spec.Auth.IdentityProviderImage); err != nil {
			return err
		}
	}

	if deploy.MigratingToCRW2_0(instance) &&
		!instance.Spec.Server.ExternalPluginRegistry &&
		instance.Spec.Server.PluginRegistryUrl == deploy.OldCrwPluginRegistryUrl {
		instance.Spec.Server.PluginRegistryUrl = ""
		if err := r.UpdateCheCRSpec(instance, "plugin registry url", instance.Spec.Server.PluginRegistryUrl); err != nil {
			return err
		}
	}

	if deploy.MigratingToCRW2_0(instance) &&
		instance.Spec.Server.CheImage == deploy.OldDefaultCodeReadyServerImageRepo {
		instance.Spec.Server.CheImage = ""
		if err := r.UpdateCheCRSpec(instance, "che image repo", instance.Spec.Server.CheImage); err != nil {
			return err
		}
	}

	if deploy.MigratingToCRW2_0(instance) &&
		instance.Spec.Server.CheImageTag == deploy.OldDefaultCodeReadyServerImageTag {
		instance.Spec.Server.CheImageTag = ""
		if err := r.UpdateCheCRSpec(instance, "che image tag", instance.Spec.Server.CheImageTag); err != nil {
			return err
		}
	}

	return nil
}
