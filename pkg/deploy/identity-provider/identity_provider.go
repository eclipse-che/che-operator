package identity_provider

import (
	"context"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/expose"
	"strings"

	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	Keycloak = "keycloak"
)

// SyncIdentityProviderToCluster instantiates the identity provider (Keycloak) in the cluster. Returns true if
// the provisioning is complete, false if requeue of the reconcile request is needed.
func SyncIdentityProviderToCluster(deployContext *deploy.DeployContext, cheHost string, protocol string, cheFlavor string) (bool, error) {
	instance := deployContext.CheCluster
	cheMultiUser := deploy.GetCheMultiUser(instance)
	tests := util.IsTestMode()
	isOpenShift := util.IsOpenShift

	if instance.Spec.Auth.ExternalIdentityProvider {
		return true, nil
	}

	if cheMultiUser == "false" {
		if util.K8sclient.IsDeploymentExists("keycloak", instance.Namespace) {
			util.K8sclient.DeleteDeployment("keycloak", instance.Namespace)
		}

		return true, nil
	}

	keycloakLabels := deploy.GetLabels(instance, "keycloak")

	serviceStatus := deploy.SyncServiceToCluster(deployContext, "keycloak", []string{"http"}, []int32{8080}, keycloakLabels)
	if !tests {
		if !serviceStatus.Continue {
			logrus.Info("Waiting on service 'keycloak' to be ready")
			if serviceStatus.Err != nil {
				logrus.Error(serviceStatus.Err)
			}

			return false, serviceStatus.Err
		}
	}

	endpoint, done, err := expose.Expose(deployContext, cheHost, Keycloak)
	if !done {
		return false, err
	}
	keycloakURL := protocol + "://" + endpoint

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

	if isOpenShift {
		doInstallOpenShiftoAuthProvider := instance.Spec.Auth.OpenShiftoAuth
		if doInstallOpenShiftoAuthProvider {
			openShiftIdentityProviderStatus := instance.Status.OpenShiftoAuthProvisioned
			if !openShiftIdentityProviderStatus {
				if err := CreateIdentityProviderItems(deployContext, cheFlavor); err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

func CreateIdentityProviderItems(deployContext *deploy.DeployContext, cheFlavor string) error {
	instance := deployContext.CheCluster
	tests := util.IsTestMode()
	isOpenShift4 := util.IsOpenShift4
	keycloakDeploymentName := KeycloakDeploymentName
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
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuthC secret name", oauthSecret); err != nil {
			return err
		}
	}

	keycloakURL := instance.Spec.Auth.IdentityProviderURL
	keycloakRealm := util.GetValue(instance.Spec.Auth.IdentityProviderRealm, cheFlavor)
	oAuthClient := deploy.NewOAuthClient(oAuthClientName, oauthSecret, keycloakURL, keycloakRealm, isOpenShift4)
	if err := createNewOauthClient(deployContext, oAuthClient); err != nil {
		return err
	}

	if !tests {
		openShiftIdentityProviderCommand, err := GetOpenShiftIdentityProviderProvisionCommand(instance, oAuthClientName, oauthSecret, isOpenShift4)
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

func createNewOauthClient(deployContext *deploy.DeployContext, oAuthClient *oauth.OAuthClient) error {
	oAuthClientFound := &oauth.OAuthClient{}
	err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: oAuthClient.Name, Namespace: oAuthClient.Namespace}, oAuthClientFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", oAuthClient.Kind, oAuthClient.Name)
		err = deployContext.ClusterAPI.Client.Create(context.TODO(), oAuthClient)
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

func reload(deployContext *deploy.DeployContext) error {
	return deployContext.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: deployContext.CheCluster.Name, Namespace: deployContext.CheCluster.Namespace},
		deployContext.CheCluster)
}
