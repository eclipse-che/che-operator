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
func (r *ReconcileChe) CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error {
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
	} else if updateIfExists {
		deploy.MergeServices(serviceFound, service)
		if err := r.client.Update(context.TODO(), serviceFound); err != nil {
			logrus.Errorf("Failed to update %s %s: %s", service.Kind, service.Name, err)
			return err
		}
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
	keycloakAdminPassword := instance.Spec.Auth.IdentityProviderPassword
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
		openShiftIdentityProviderCommand, err := deploy.GetOpenShiftIdentityProviderProvisionCommand(instance, oAuthClientName, oauthSecret, keycloakAdminPassword, isOpenShift4)
		if err != nil {
			logrus.Errorf("Failed to build identity provider provisioning command")
			return err
		}
		podToExec, err := k8sclient.GetDeploymentPod(keycloakDeploymentName, instance.Namespace)
		if err != nil {
			logrus.Errorf("Failed to retrieve pod name. Further exec will fail")
			return err
		}
		provisioned := ExecIntoPod(podToExec, openShiftIdentityProviderCommand, "create OpenShift identity provider", instance.Namespace)
		if provisioned {
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
		return nil
	}
	return nil
}

func (r *ReconcileChe) CreateTLSSecret(instance *orgv1.CheCluster, url string, name string) (err error) {
	// create a secret with either router tls cert (or OpenShift API crt) when on OpenShift infra
	// and router is configured with a self signed certificate
	// this secret is used by CRW server to reach RH SSO TLS endpoint
	secret := &corev1.Secret{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, secret); err != nil && errors.IsNotFound(err) {
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

	cheFlavor := util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor)
	if len(instance.Spec.Server.CheFlavor) < 1 {
		instance.Spec.Server.CheFlavor = cheFlavor
		if err := r.UpdateCheCRSpec(instance, "installation flavor", cheFlavor); err != nil {
			return err
		}
	}

	cheMultiUser := deploy.GetCheMultiUser(instance)
	if cheMultiUser == "true" {
		chePostgresPassword := util.GetValue(instance.Spec.Database.ChePostgresPassword, util.GeneratePasswd(12))
		if len(instance.Spec.Database.ChePostgresPassword) < 1 {
			instance.Spec.Database.ChePostgresPassword = chePostgresPassword
			if err := r.UpdateCheCRSpec(instance, "auto-generated CheCluster DB password", "password-hidden"); err != nil {
				return err
			}

		}
		keycloakPostgresPassword := util.GetValue(instance.Spec.Auth.IdentityProviderPostgresPassword, util.GeneratePasswd(12))
		if len(instance.Spec.Auth.IdentityProviderPostgresPassword) < 1 {
			instance.Spec.Auth.IdentityProviderPostgresPassword = keycloakPostgresPassword
			keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
			if err != nil {
				logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating passwd")
			} else {
				keycloakPostgresPassword = util.GetDeploymentEnv(keycloakDeployment, "DB_PASSWORD")
			}
			if err := r.UpdateCheCRSpec(instance, "auto-generated Keycloak DB password", "password-hidden"); err != nil {
				return err
			}
		}
		if len(instance.Spec.Auth.IdentityProviderPassword) < 1 {
			keycloakAdminPassword := util.GetValue(instance.Spec.Auth.IdentityProviderPassword, util.GeneratePasswd(12))
			keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
			if err != nil {
				logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating passwd")
			} else {
				keycloakAdminPassword = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_PASSWORD")
			}
			instance.Spec.Auth.IdentityProviderPassword = keycloakAdminPassword
			if err := r.UpdateCheCRSpec(instance, "Keycloak admin password", "password hidden"); err != nil {
				return err
			}
		}
		if len(instance.Spec.Auth.IdentityProviderAdminUserName) < 1 {
			keycloakAdminUserName := util.GetValue(instance.Spec.Auth.IdentityProviderAdminUserName, "admin")
			keycloakDeployment, err := r.GetEffectiveDeployment(instance, "keycloak")
			if err != nil {
				logrus.Info("Disregard the error. No existing Identity provider deployment found. Generating admin username")
			} else {
				keycloakAdminUserName = util.GetDeploymentEnv(keycloakDeployment, "SSO_ADMIN_USERNAME")
			}
			instance.Spec.Auth.IdentityProviderAdminUserName = keycloakAdminUserName
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
