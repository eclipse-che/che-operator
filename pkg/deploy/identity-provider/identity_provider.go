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
package identity_provider

import (
	"context"
	"errors"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var (
	oAuthClientDiffOpts = cmpopts.IgnoreFields(oauth.OAuthClient{}, "TypeMeta", "ObjectMeta")
	syncItems           = []func(*deploy.DeployContext) (bool, error){
		syncService,
		syncExposure,
		SyncKeycloakDeploymentToCluster,
		syncKeycloakResources,
		syncOpenShiftIdentityProvider,
		SyncGitHubOAuth,
	}

	keycloakUpdated = false
	keycloakCheHost = ""
)

// SyncIdentityProviderToCluster instantiates the identity provider (Keycloak) in the cluster. Returns true if
// the provisioning is complete, false if requeue of the reconcile request is needed.
func SyncIdentityProviderToCluster(deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster
	if util.IsOpenShift && deployContext.CheCluster.IsNativeUserModeEnabled() {
		return syncNativeIdentityProviderItems(deployContext)
	} else if cr.Spec.Auth.ExternalIdentityProvider {
		return true, nil
	}

	for _, syncItem := range syncItems {
		provisioned, err := syncItem(deployContext)
		if !util.IsTestMode() {
			if !provisioned {
				return false, err
			}
		}
	}

	return true, nil
}

func syncService(deployContext *deploy.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		deployContext,
		deploy.IdentityProviderName,
		[]string{"http"},
		[]int32{8080},
		deploy.IdentityProviderName)
}

func syncExposure(deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster

	protocol := (map[bool]string{
		true:  "https",
		false: "http"})[cr.Spec.Server.TlsSupport]
	endpoint, done, err := expose.Expose(
		deployContext,
		deploy.IdentityProviderName,
		cr.Spec.Auth.IdentityProviderRoute,
		cr.Spec.Auth.IdentityProviderIngress,
		createGatewayConfig(deployContext.CheCluster))
	if !done {
		return false, err
	}

	keycloakURL := protocol + "://" + endpoint
	if cr.Spec.Auth.IdentityProviderURL != keycloakURL {
		cr.Spec.Auth.IdentityProviderURL = keycloakURL
		if err := deploy.UpdateCheCRSpec(deployContext, "Keycloak URL", keycloakURL); err != nil {
			return false, err
		}

		cr.Status.KeycloakURL = keycloakURL
		if err := deploy.UpdateCheCRStatus(deployContext, "Keycloak URL", keycloakURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func syncKeycloakResources(deployContext *deploy.DeployContext) (bool, error) {
	if !util.IsTestMode() {
		cr := deployContext.CheCluster
		if !cr.Status.KeycloakProvisoned {
			if err := ProvisionKeycloakResources(deployContext); err != nil {
				return false, err
			}

			for {
				cr.Status.KeycloakProvisoned = true
				if err := deploy.UpdateCheCRStatus(deployContext, "status: provisioned with Keycloak", "true"); err != nil &&
					apierrors.IsConflict(err) {

					deploy.ReloadCheClusterCR(deployContext)
					continue
				}
				break
			}
		}

		// Updates keycloak if chehost has been changed
		if !keycloakUpdated || keycloakCheHost != deployContext.CheCluster.Spec.Server.CheHost {
			if _, err := util.K8sclient.ExecIntoPod(
				deployContext.CheCluster,
				deploy.IdentityProviderName,
				GetKeycloakUpdateCommand,
				"Update redirect URI-s and webOrigins"); err != nil {
				return false, err
			} else {
				keycloakUpdated = true
				keycloakCheHost = deployContext.CheCluster.Spec.Server.CheHost
			}
		}
	}

	return true, nil
}

func syncOpenShiftIdentityProvider(deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster
	if util.IsOpenShift && cr.IsOpenShiftOAuthEnabled() {
		return SyncOpenShiftIdentityProviderItems(deployContext)
	}
	return true, nil
}

func syncNativeIdentityProviderItems(deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster

	if err := resolveOpenshiftOAuthClientName(deployContext); err != nil {
		return false, err
	}
	if err := resolveOpenshiftOAuthClientSecret(deployContext); err != nil {
		return false, err
	}

	redirectURIs := []string{"https://" + cr.Spec.Server.CheHost + "/oauth/callback"}

	oAuthClient := deploy.GetOAuthClientSpec(cr.Spec.Auth.OAuthClientName, cr.Spec.Auth.OAuthSecret, redirectURIs)
	provisioned, err := deploy.Sync(deployContext, oAuthClient, oAuthClientDiffOpts)
	if !provisioned {
		return false, err
	}

	return true, nil
}

func SyncOpenShiftIdentityProviderItems(deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster

	if err := resolveOpenshiftOAuthClientName(deployContext); err != nil {
		return false, err
	}
	if err := resolveOpenshiftOAuthClientSecret(deployContext); err != nil {
		return false, err
	}

	keycloakURL := cr.Spec.Auth.IdentityProviderURL
	cheFlavor := deploy.DefaultCheFlavor(cr)
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	oAuthClient := deploy.GetKeycloakOAuthClientSpec(cr.Spec.Auth.OAuthClientName, cr.Spec.Auth.OAuthSecret, keycloakURL, keycloakRealm, util.IsOpenShift4)
	provisioned, err := deploy.Sync(deployContext, oAuthClient, oAuthClientDiffOpts)
	if !provisioned {
		return false, err
	}

	if !util.IsTestMode() {
		if !cr.Status.OpenShiftoAuthProvisioned {
			// note that this uses the instance.Spec.Auth.IdentityProviderRealm and instance.Spec.Auth.IdentityProviderClientId.
			// because we're not doing much of a change detection on those fields, we can't react on them changing here.
			_, err := util.K8sclient.ExecIntoPod(
				cr,
				deploy.IdentityProviderName,
				func(cr *orgv1.CheCluster) (string, error) {
					return GetOpenShiftIdentityProviderProvisionCommand(cr, cr.Spec.Auth.OAuthClientName, cr.Spec.Auth.OAuthSecret)
				},
				"Create OpenShift identity provider")
			if err != nil {
				return false, err
			}

			for {
				cr.Status.OpenShiftoAuthProvisioned = true
				if err := deploy.UpdateCheCRStatus(deployContext, "status: provisioned with OpenShift identity provider", "true"); err != nil &&
					apierrors.IsConflict(err) {

					deploy.ReloadCheClusterCR(deployContext)
					continue
				}
				break
			}
		}
	}
	return true, nil
}

func resolveOpenshiftOAuthClientName(deployContext *deploy.DeployContext) error {
	cr := deployContext.CheCluster
	oAuthClientName := cr.Spec.Auth.OAuthClientName
	if len(oAuthClientName) < 1 {
		oAuthClientName = cr.Name + "-openshift-identity-provider-" + strings.ToLower(util.GeneratePasswd(6))
		cr.Spec.Auth.OAuthClientName = oAuthClientName
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuthClient name", oAuthClientName); err != nil {
			return err
		}
	}
	return nil
}

func resolveOpenshiftOAuthClientSecret(deployContext *deploy.DeployContext) error {
	cr := deployContext.CheCluster
	oauthSecret := cr.Spec.Auth.OAuthSecret
	if len(oauthSecret) < 1 {
		oauthSecret = util.GeneratePasswd(12)
		cr.Spec.Auth.OAuthSecret = oauthSecret
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuth secret name", oauthSecret); err != nil {
			return err
		}
	}
	return nil
}

// SyncGitHubOAuth provisions GitHub OAuth if secret with annotation
// `che.eclipse.org/github-oauth-credentials=true` or `che.eclipse.org/oauth-scm-configuration=github`
// is mounted into a container
func SyncGitHubOAuth(deployContext *deploy.DeployContext) (bool, error) {
	// get legacy secret
	legacySecrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.IdentityProviderName + "-secret",
	}, map[string]string{
		deploy.CheEclipseOrgGithubOAuthCredentials: "true",
	})
	if err != nil {
		return false, err
	}

	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "github",
	})

	if err != nil {
		return false, err
	} else if len(secrets)+len(legacySecrets) > 1 {
		return false, errors.New("More than 1 GitHub OAuth configuration secrets found")
	}

	isGitHubOAuthCredentialsExists := len(secrets) == 1 || len(legacySecrets) == 1
	cr := deployContext.CheCluster

	if isGitHubOAuthCredentialsExists {
		if !cr.Status.GitHubOAuthProvisioned {
			if !util.IsTestMode() {
				_, err := util.K8sclient.ExecIntoPod(
					cr,
					deploy.IdentityProviderName,
					func(cr *orgv1.CheCluster) (string, error) {
						return GetGitHubIdentityProviderCreateCommand(deployContext)
					},
					"Create GitHub OAuth")
				if err != nil {
					return false, err
				}
			}

			cr.Status.GitHubOAuthProvisioned = true
			if err := deploy.UpdateCheCRStatus(deployContext, "status: GitHub OAuth provisioned", "true"); err != nil {
				return false, err
			}
		}
	} else {
		if cr.Status.GitHubOAuthProvisioned {
			if !util.IsTestMode() {
				_, err := util.K8sclient.ExecIntoPod(
					cr,
					deploy.IdentityProviderName,
					func(cr *orgv1.CheCluster) (string, error) {
						return GetIdentityProviderDeleteCommand(cr, "github")
					},
					"Delete GitHub OAuth")
				if err != nil {
					return false, err
				}
			}

			cr.Status.GitHubOAuthProvisioned = false
			if err := deploy.UpdateCheCRStatus(deployContext, "status: GitHub OAuth provisioned", "false"); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func ReconcileIdentityProvider(deployContext *deploy.DeployContext) (deleted bool, err error) {
	if !deployContext.CheCluster.IsOpenShiftOAuthEnabled() && deployContext.CheCluster.Status.OpenShiftoAuthProvisioned == true {
		keycloakDeployment := &appsv1.Deployment{}
		if err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: deploy.IdentityProviderName, Namespace: deployContext.CheCluster.Namespace}, keycloakDeployment); err != nil {
			logrus.Errorf("Deployment %s not found: %s", keycloakDeployment.Name, err.Error())
		}

		providerName := "openshift-v3"
		if util.IsOpenShift4 {
			providerName = "openshift-v4"
		}
		_, err := util.K8sclient.ExecIntoPod(
			deployContext.CheCluster,
			keycloakDeployment.Name,
			func(cr *orgv1.CheCluster) (string, error) {
				return GetIdentityProviderDeleteCommand(deployContext.CheCluster, providerName)
			},
			"delete OpenShift identity provider")
		if err == nil {
			oAuthClient := &oauth.OAuthClient{}
			oAuthClientName := deployContext.CheCluster.Spec.Auth.OAuthClientName
			if err := deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName, Namespace: ""}, oAuthClient); err != nil {
				logrus.Errorf("OAuthClient %s not found: %s", oAuthClient.Name, err.Error())
			}
			if err := deployContext.ClusterAPI.NonCachingClient.Delete(context.TODO(), oAuthClient); err != nil {
				logrus.Errorf("Failed to delete %s %s: %s", oAuthClient.Kind, oAuthClient.Name, err.Error())
			}
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func createGatewayConfig(cheCluster *orgv1.CheCluster) *gateway.TraefikConfig {
	cfg := gateway.CreateCommonTraefikConfig(
		deploy.IdentityProviderName,
		"PathPrefix(`/auth`)",
		10,
		"http://"+deploy.IdentityProviderName+":8080",
		[]string{})

	if util.IsOpenShift && cheCluster.IsNativeUserModeEnabled() {
		cfg.AddAuthHeaderRewrite(deploy.IdentityProviderName)
	}

	return cfg
}
