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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

func (r *ReconcileChe) CreateNewDeployment(instance *orgv1.CheCluster, deployment *appsv1.Deployment) error {
	if err := controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
		logrus.Errorf("An error occurred: %s", err)
		return err
	}
	deploymentFound := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, deploymentFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", deployment.Kind, deployment.Name)
		err = r.client.Create(context.TODO(), deployment)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", deployment.Kind, deployment.Name, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred: %s", err)
		return err
	}
	return nil
}

func (r *ReconcileChe) CreateNewConfigMap(instance *orgv1.CheCluster, configMap *corev1.ConfigMap) error {
	if err := controllerutil.SetControllerReference(instance, configMap, r.scheme); err != nil {
		logrus.Errorf("An error occurred: %s", err)
		return err
	}
	configMapFound := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, configMapFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", configMap.Kind, configMap.Name)
		err = r.client.Create(context.TODO(), configMap)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", configMap.Kind, configMap.Name, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred: %s", err)

		return err
	}
	return nil
}

func (r *ReconcileChe) CreateServiceAccount(cr *orgv1.CheCluster, serviceAccount *corev1.ServiceAccount) error {
	if err := controllerutil.SetControllerReference(cr, serviceAccount, r.scheme); err != nil {
		return err
	}
	serviceAccountFound := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, serviceAccountFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", serviceAccount.Kind, serviceAccount.Name)
		err = r.client.Create(context.TODO(), serviceAccount)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", serviceAccount.Name, serviceAccount.Kind, err)
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileChe) CreateNewRole(instance *orgv1.CheCluster, role *rbac.Role) error {
	if err := controllerutil.SetControllerReference(instance, role, r.scheme); err != nil {
		return err
	}
	roleFound := &rbac.Role{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, roleFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", role.Kind, role.Name)
		err = r.client.Create(context.TODO(), role)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", role.Name, role.Kind, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred: %s", err)
		return err
	}
	return nil
}

func (r *ReconcileChe) CreateNewIngress(instance *orgv1.CheCluster, ingress *v1beta1.Ingress) error {
	if err := controllerutil.SetControllerReference(instance, ingress, r.scheme); err != nil {
		logrus.Errorf("An error occurred %s", err)
		return err
	}
	ingressFound := &v1beta1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, ingressFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object %s, name: %s", ingress.Kind, ingress.Name)
		if err := r.client.Create(context.TODO(), ingress); err != nil {
			logrus.Errorf("Failed to create %s %s: %s", ingress.Kind, ingress.Name, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred %s", err)

		return err
	}
	return nil
}

func (r *ReconcileChe) CreateNewRoute(instance *orgv1.CheCluster, route *routev1.Route) error {
	if err := controllerutil.SetControllerReference(instance, route, r.scheme); err != nil {
		logrus.Errorf("An error occurred %s", err)
		return err
	}
	routeFound := &routev1.Route{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: route.Name, Namespace: route.Namespace}, routeFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object %s, name: %s", route.Kind, route.Name)
		err = r.client.Create(context.TODO(), route)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", route.Kind, route.Name, err)
			return err
		}
		// Route created successfully - don't requeue
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred %s", err)
		return err
	}
	return nil
}

func (r *ReconcileChe) CreateNewSecret(instance *orgv1.CheCluster, secret *corev1.Secret) error {
	if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
		logrus.Errorf("An error occurred: %s", err)
		return err
	}
	deploymentFound := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, deploymentFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", secret.Kind, secret.Name)
		err = r.client.Create(context.TODO(), secret)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", secret.Kind, secret.Name, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred: %s", err)

		return err
	}
	return nil
}

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

// CreateService creates a service with a given name, port, selector and labels
func (r *ReconcileChe) CreateService(cr *orgv1.CheCluster, service *corev1.Service) error {
	if err := controllerutil.SetControllerReference(cr, service, r.scheme); err != nil {
		logrus.Errorf("An error occurred %s", err)
		return err
	}
	serviceFound := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, serviceFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object %s, name: %s", service.Kind, service.Name)
		err = r.client.Create(context.TODO(), service)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", service.Kind, service.Name, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred %s", err)
		return err
	}
	return nil
}

func (r *ReconcileChe) CreatePVC(instance *orgv1.CheCluster, pvc *corev1.PersistentVolumeClaim) error {
	// Set CheCluster instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pvc, r.scheme); err != nil {
		return err
	}
	pvcFound := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, pvcFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object %s, name: %s", pvc.Kind, pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	return nil

}

func (r *ReconcileChe) CreateNewRoleBinding(instance *orgv1.CheCluster, roleBinding *rbac.RoleBinding) error {
	if err := controllerutil.SetControllerReference(instance, roleBinding, r.scheme); err != nil {
		return err
	}
	roleBindingFound := &rbac.RoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, roleBindingFound)
	if err != nil && errors.IsNotFound(err) {
		logrus.Infof("Creating a new object: %s, name: %s", roleBinding.Kind, roleBinding.Name)
		err = r.client.Create(context.TODO(), roleBinding)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", roleBinding.Name, roleBinding.Kind, err)
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
	keycloakAdminPassword := instance.Spec.Auth.KeycloakAdminPassword
	oAuthClientName := instance.Spec.Auth.OauthClientName
	if len(oAuthClientName) < 1 {
		oAuthClientName = instance.Name + "-openshift-identity-provider-" + strings.ToLower(util.GeneratePasswd(6))
		instance.Spec.Auth.OauthClientName = oAuthClientName
		if err := r.UpdateCheCRSpec(instance, "oAuthClient name", oAuthClientName); err != nil {
			return err
		}
	}
	oauthSecret := instance.Spec.Auth.OauthSecret
	if len(oauthSecret) < 1 {
		oauthSecret = util.GeneratePasswd(12)
		instance.Spec.Auth.OauthSecret = oauthSecret
		if err := r.UpdateCheCRSpec(instance, "oAuthC secret name", oauthSecret); err != nil {
			return err
		}
	}
	keycloakURL := instance.Spec.Auth.KeycloakURL
	keycloakRealm := util.GetValue(instance.Spec.Auth.KeycloakRealm, cheFlavor)
	oAuthClient := deploy.NewOAuthClient(oAuthClientName, oauthSecret, keycloakURL, keycloakRealm, isOpenShift4)
	if err := r.CreateNewOauthClient(instance, oAuthClient); err != nil {
		return err
	}

	if !tests {
		openShiftIdentityProviderCommand := deploy.GetOpenShiftIdentityProviderProvisionCommand(instance, oAuthClientName, oauthSecret, keycloakAdminPassword, isOpenShift4)
		podToExec, err := k8sclient.GetDeploymentPod(keycloakDeploymentName, instance.Namespace)
		if err != nil {
			logrus.Errorf("Failed to retrieve pod name. Further exec will fail")
			return err
		}
		provisioned := ExecIntoPod(podToExec, openShiftIdentityProviderCommand, "create OpenShift identity provider", instance.Namespace)
		if provisioned {
			for {
				instance.Status.OpenShiftoAuthProvisioned = true
				if err := r.UpdateCheCRStatus(instance, "status: provisioned with OpenShift identity provider", "true"); err != nil {
					instance, _ = r.GetCR(request)
				} else {
					break
				}
			}
		}
		return nil
	}
	return nil
}

func (r *ReconcileChe) CreateTLSSecret(instance *orgv1.CheCluster, url string, name string) (err error) {
	// create a secret with either router tls cert (or OpenShift API crt) when on OpenShift infra
	// and router is configured with a self signed certificate
	// this secret is used by CRW server to reach RH SSO TLS endpoint
	secret := &corev1.Secret{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, secret);
		err != nil && errors.IsNotFound(err) {
		crt, err := r.GetEndpointTlsCrt(instance, url)
		if err != nil {
			logrus.Errorf("Failed to extract crt. Failed to create a secret with a self signed crt: %s", err)
			return err
		} else {
			secret := deploy.NewSecret(instance, name, crt)
			if err := r.CreateNewSecret(instance, secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileChe) GenerateAndSaveFields(instance *orgv1.CheCluster, request reconcile.Request) (err error) {

	chePostgresPassword := util.GetValue(instance.Spec.Database.ChePostgresPassword, util.GeneratePasswd(12))
	if len(instance.Spec.Database.ChePostgresPassword) < 1 {
		instance.Spec.Database.ChePostgresPassword = chePostgresPassword
		if err := r.UpdateCheCRSpec(instance, "auto-generated CheCluster DB password", "password-hidden"); err != nil {
			return err
		}

	}
	keycloakPostgresPassword := util.GetValue(instance.Spec.Auth.KeycloakPostgresPassword, util.GeneratePasswd(12))
	if len(instance.Spec.Auth.KeycloakPostgresPassword) < 1 {
		instance.Spec.Auth.KeycloakPostgresPassword = keycloakPostgresPassword
		keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
		if err != nil {
			logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating passwd")
		} else {
			keycloakPostgresPassword = r.GetDeploymentEnv(keycloakDeployment, "DB_PASSWORD")
		}
		if err := r.UpdateCheCRSpec(instance, "auto-generated Keycloak DB password", "password-hidden"); err != nil {
			return err
		}
	}
	if len(instance.Spec.Auth.KeycloakAdminPassword) < 1 {
		keycloakAdminPassword := util.GetValue(instance.Spec.Auth.KeycloakAdminPassword, util.GeneratePasswd(12))
		keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
		if err != nil {
			logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating passwd")
		} else {
			keycloakAdminPassword = r.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_PASSWORD")
		}
		instance.Spec.Auth.KeycloakAdminPassword = keycloakAdminPassword
		if err := r.UpdateCheCRSpec(instance, "Keycloak admin password", "password hidden"); err != nil {
			return err
		}
	}
	if len(instance.Spec.Auth.KeycloakAdminUserName) < 1 {
		keycloakAdminUserName := util.GetValue(instance.Spec.Auth.KeycloakAdminUserName, "admin")
		keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
		if err != nil {
			logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating admin username")
		} else {
			keycloakAdminUserName = r.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_USERNAME")
		}
		instance.Spec.Auth.KeycloakAdminUserName = keycloakAdminUserName
		if err := r.UpdateCheCRSpec(instance, "Keycloak admin username", keycloakAdminUserName); err != nil {
			return err
		}
	}
	chePostgresUser := util.GetValue(instance.Spec.Database.ChePostgresUser, "pgche")
	if len(instance.Spec.Database.ChePostgresUser) < 1 {
		instance.Spec.Database.ChePostgresUser = chePostgresUser
		if err := r.UpdateCheCRSpec(instance, "Postgres User", chePostgresUser); err != nil {
			return err
		}
	}
	chePostgresDb := util.GetValue(instance.Spec.Database.ChePostgresDb, "dbche")
	if len(instance.Spec.Database.ChePostgresDb) < 1 {
		instance.Spec.Database.ChePostgresDb = chePostgresDb
		if err := r.UpdateCheCRSpec(instance, "Postgres DB", chePostgresDb); err != nil {
			return err
		}
	}
	chePostgresHostName := util.GetValue(instance.Spec.Database.ChePostgresDBHostname, deploy.DefaultChePostgresHostName)
	if len(instance.Spec.Database.ChePostgresDBHostname) < 1 {
		instance.Spec.Database.ChePostgresDBHostname = chePostgresHostName
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
	cheFlavor := util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor)
	if len(instance.Spec.Server.CheFlavor) < 1 {
		instance.Spec.Server.CheFlavor = cheFlavor
		if err := r.UpdateCheCRSpec(instance, "installation flavor", cheFlavor); err != nil {
			return err
		}
	}
	defaultPostgresImage := deploy.DefaultPostgresUpstreamImage
	if cheFlavor == "codeready" {
		defaultPostgresImage = deploy.DefaultPostgresImage

	}
	postgresImage := util.GetValue(instance.Spec.Database.PostgresImage, defaultPostgresImage)
	if len(instance.Spec.Database.PostgresImage) < 1 {
		instance.Spec.Database.PostgresImage = postgresImage
		if err := r.UpdateCheCRSpec(instance, "DB image:tag", postgresImage); err != nil {
			return err
		}
	}

	defaultKeycloakImage := deploy.DefaultKeycloakUpstreamImage
	if cheFlavor == "codeready" {
		defaultKeycloakImage = deploy.DefaultKeycloakImage

	}

	keycloakImage := util.GetValue(instance.Spec.Auth.KeycloakImage, defaultKeycloakImage)
	if len(instance.Spec.Auth.KeycloakImage) < 1 {
		instance.Spec.Auth.KeycloakImage = keycloakImage
		keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
		if err != nil {
			logrus.Info("Disregard the error. No existing Identity provider deployment found. Using default image")
		} else {
			keycloakImage = keycloakDeployment.Spec.Template.Spec.Containers[0].Image
		}
		if err := r.UpdateCheCRSpec(instance, "Keycloak image:tag", keycloakImage); err != nil {
			return err
		}
	}
	keycloakRealm := util.GetValue(instance.Spec.Auth.KeycloakRealm, cheFlavor)
	if len(instance.Spec.Auth.KeycloakRealm) < 1 {
		instance.Spec.Auth.KeycloakRealm = keycloakRealm
		if err := r.UpdateCheCRSpec(instance, "Keycloak realm", keycloakRealm); err != nil {
			return err
		}
	}
	keycloakClientId := util.GetValue(instance.Spec.Auth.KeycloakClientId, cheFlavor+"-public")
	if len(instance.Spec.Auth.KeycloakClientId) < 1 {
		instance.Spec.Auth.KeycloakClientId = keycloakClientId

		if err := r.UpdateCheCRSpec(instance, "Keycloak client ID", keycloakClientId); err != nil {
			return err
		}
	}
	pluginRegistryUrl := util.GetValue(instance.Spec.Server.PluginRegistryUrl, deploy.DefaultUpstreamPluginRegistryUrl)
	if cheFlavor == "codeready" {
		pluginRegistryUrl = deploy.DefaultPluginRegistryUrl
	}

	if len(instance.Spec.Server.PluginRegistryUrl) < 1 {
		instance.Spec.Server.PluginRegistryUrl = pluginRegistryUrl
		if err := r.UpdateCheCRSpec(instance, "plugin registry URL", pluginRegistryUrl); err != nil {
			return err
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
	defaultPVCJobsImage := deploy.DefaultPvcJobsUpstreamImage
	if cheFlavor == "codeready" {
		defaultPVCJobsImage = deploy.DefaultPvcJobsImage
	}
	pvcJobsImage := util.GetValue(instance.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	if len(instance.Spec.Storage.PvcJobsImage) < 1 {
		instance.Spec.Storage.PvcJobsImage = pvcJobsImage
		if err := r.UpdateCheCRSpec(instance, "pvc jobs image", pvcJobsImage); err != nil {
			return err
		}
	}
	return nil
}
