//
// Copyright (c) 2020-2020 Red Hat, Inc.
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
	"fmt"
	"strings"

	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/expose"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var (
	oAuthClientDiffOpts = cmpopts.IgnoreFields(oauth.OAuthClient{}, "TypeMeta", "ObjectMeta")
)

// SyncIdentityProviderToCluster instantiates the identity provider (Keycloak) in the cluster. Returns true if
// the provisioning is complete, false if requeue of the reconcile request is needed.
func SyncIdentityProviderToCluster(deployContext *deploy.DeployContext) (bool, error) {
	instance := deployContext.CheCluster
	cheHost := instance.Spec.Server.CheHost
	protocol := "http"
	if instance.Spec.Server.TlsSupport {
		protocol = "https"
	}
	cheFlavor := deploy.DefaultCheFlavor(instance)
	cheMultiUser := deploy.GetCheMultiUser(instance)
	tests := util.IsTestMode()
	isOpenShift := util.IsOpenShift

	if instance.Spec.Auth.ExternalIdentityProvider {
		return true, nil
	}

	if cheMultiUser == "false" {
		if util.K8sclient.IsDeploymentExists(deploy.IdentityProviderName, instance.Namespace) {
			util.K8sclient.DeleteDeployment(deploy.IdentityProviderName, instance.Namespace)
		}

		return true, nil
	}

	serviceStatus := deploy.SyncServiceToCluster(deployContext, deploy.IdentityProviderName, []string{"http"}, []int32{8080}, deploy.IdentityProviderName)
	if !tests {
		if !serviceStatus.Continue {
			logrus.Infof("Waiting on service '%s' to be ready", deploy.IdentityProviderName)
			if serviceStatus.Err != nil {
				logrus.Error(serviceStatus.Err)
			}

			return false, serviceStatus.Err
		}
	}

	additionalLabels := (map[bool]string{true: instance.Spec.Auth.IdentityProviderRoute.Labels, false: instance.Spec.Auth.IdentityProviderIngress.Labels})[util.IsOpenShift]
	endpoint, done, err := expose.Expose(deployContext, cheHost, deploy.IdentityProviderName, additionalLabels, deploy.IdentityProviderName)
	if !done {
		return false, err
	}
	keycloakURL := protocol + "://" + endpoint
	deployContext.InternalService.KeycloakHost = fmt.Sprintf("%s://%s.%s.svc:%d", "http", deploy.IdentityProviderName, deployContext.CheCluster.Namespace, 8080)

	if instance.Spec.Auth.IdentityProviderURL != keycloakURL {
		instance.Spec.Auth.IdentityProviderURL = keycloakURL
		if err := deploy.UpdateCheCRSpec(deployContext, "Keycloak URL", keycloakURL); err != nil {
			return false, err
		}

		instance.Status.KeycloakURL = keycloakURL
		if err := deploy.UpdateCheCRStatus(deployContext, "Keycloak URL", keycloakURL); err != nil {
			return false, err
		}
	}

	deploymentStatus := SyncKeycloakDeploymentToCluster(deployContext)
	if !tests {
		if !deploymentStatus.Continue {
			logrus.Info("Waiting on deployment 'keycloak' to be ready")
			if deploymentStatus.Err != nil {
				logrus.Error(deploymentStatus.Err)
			}

			return false, deploymentStatus.Err
		}
	}

	if !tests {
		if !instance.Status.KeycloakProvisoned {
			if err := ProvisionKeycloakResources(deployContext); err != nil {
				logrus.Error(err)
				return false, err
			}

			for {
				instance.Status.KeycloakProvisoned = true
				if err := deploy.UpdateCheCRStatus(deployContext, "status: provisioned with Keycloak", "true"); err != nil &&
					errors.IsConflict(err) {

					reload(deployContext)
					continue
				}
				break
			}
		}
	}

	if isOpenShift && util.IsOAuthEnabled(instance) {
		if err := SyncIdentityProviderItems(deployContext, cheFlavor); err != nil {
			return false, err
		}
	}

	return true, nil
}

func SyncIdentityProviderItems(deployContext *deploy.DeployContext, cheFlavor string) error {
	instance := deployContext.CheCluster
	tests := util.IsTestMode()
	isOpenShift4 := util.IsOpenShift4
	oAuthClientName := instance.Spec.Auth.OAuthClientName
	if len(oAuthClientName) < 1 {
		oAuthClientName = instance.Name + "-openshift-identity-provider-" + strings.ToLower(util.GeneratePasswd(6))
		instance.Spec.Auth.OAuthClientName = oAuthClientName
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuthClient name", oAuthClientName); err != nil {
			return err
		}
	}
	oauthSecret := instance.Spec.Auth.OAuthSecret
	if len(oauthSecret) < 1 {
		oauthSecret = util.GeneratePasswd(12)
		instance.Spec.Auth.OAuthSecret = oauthSecret
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuth secret name", oauthSecret); err != nil {
			return err
		}
	}

	keycloakURL := instance.Spec.Auth.IdentityProviderURL
	keycloakRealm := util.GetValue(instance.Spec.Auth.IdentityProviderRealm, cheFlavor)
	oAuthClient := deploy.NewOAuthClient(oAuthClientName, oauthSecret, keycloakURL, keycloakRealm, isOpenShift4)
	if _, err := deploy.Sync(deployContext, oAuthClient, oAuthClientDiffOpts); err != nil {
		return err
	}

	if !tests && !instance.Status.OpenShiftoAuthProvisioned {
		// note that this uses the instance.Spec.Auth.IdentityProviderRealm and instance.Spec.Auth.IdentityProviderClientId.
		// because we're not doing much of a change detection on those fields, we can't react on them changing here.
		openShiftIdentityProviderCommand, err := GetOpenShiftIdentityProviderProvisionCommand(instance, oAuthClientName, oauthSecret, isOpenShift4)
		if err != nil {
			logrus.Errorf("Failed to build identity provider provisioning command")
			return err
		}
		podToExec, err := util.K8sclient.GetDeploymentPod(deploy.IdentityProviderName, instance.Namespace)
		if err != nil {
			logrus.Errorf("Failed to retrieve pod name. Further exec will fail")
			return err
		}
		_, err = util.K8sclient.ExecIntoPod(podToExec, openShiftIdentityProviderCommand, "create OpenShift identity provider", instance.Namespace)
		if err == nil {
			for {
				instance.Status.OpenShiftoAuthProvisioned = true
				if err := deploy.UpdateCheCRStatus(deployContext, "status: provisioned with OpenShift identity provider", "true"); err != nil &&
					errors.IsConflict(err) {

					reload(deployContext)
					continue
				}
				break
			}
		}
	}
	return nil
}

func reload(deployContext *deploy.DeployContext) error {
	return deployContext.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: deployContext.CheCluster.Name, Namespace: deployContext.CheCluster.Namespace},
		deployContext.CheCluster)
}
