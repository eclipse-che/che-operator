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
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileChe) UpdateCheCRStatus(instance *orgv1.CheCluster, updatedField string, value string) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", instance.Name, updatedField, value)
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		logrus.Warnf("Failed to update %s CR. Fetching the latest CR version: %s", instance.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", instance.Name)
	return nil
}

func (r *ReconcileChe) UpdateCheCRSpec(instance *orgv1.CheCluster, updatedField string, value string) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", instance.Name, updatedField, value)
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		logrus.Warnf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", instance.Name)
	return nil
}

// UpdateConfigMap compares existing ConfigMap retrieved from API with a current ConfigMap
// i.e. ConfigMap.Data consuming current CheCluster.Spec fields, and updates an existing
// ConfigMap with up-to-date .Data.
func (r *ReconcileChe) UpdateConfigMap(instance *orgv1.CheCluster) (updated bool, err error) {

	activeConfigMap := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: instance.Namespace}, activeConfigMap); err != nil {
		logrus.Errorf("ConfigMap %s not found: %s", activeConfigMap.Name, err)
	}
	// compare ConfigMap.Data with current CM on server
	cheEnv := deploy.GetConfigMapData(instance)
	cm := deploy.NewCheConfigMap(instance, cheEnv)
	equal := reflect.DeepEqual(cm.Data, activeConfigMap.Data)
	if !equal {

		logrus.Infof("Updating %s ConfigMap", activeConfigMap.Name)
		if err := controllerutil.SetControllerReference(instance, cm, r.scheme); err != nil {
			logrus.Errorf("Failed to set OwnersReference for %s %s: %s", activeConfigMap.Kind, activeConfigMap.Name, err)
			return false, err
		}
		if err := r.client.Update(context.TODO(), cm); err != nil {
			logrus.Errorf("Failed to update %s %s: %s", activeConfigMap.Kind, activeConfigMap.Name, err)
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *ReconcileChe) ReconcileTLSObjects(instance *orgv1.CheCluster, request reconcile.Request, cheFlavor string, tlsSupport bool, isOpenShift bool) (updated bool, err error) {

	updateRegistryRoute := func(registryType string) (bool, error) {
		registryName := registryType + "-registry"
		if !isOpenShift {
			currentRegistryIngress := r.GetEffectiveIngress(instance, registryName)
			if currentRegistryIngress == nil {
				return false, err
			}
			logrus.Infof("Deleting ingress %s", currentRegistryIngress.Name)
			if err := r.client.Delete(context.TODO(), currentRegistryIngress); err != nil {
				logrus.Errorf("Failed to delete %s ingress: %s", currentRegistryIngress.Name, err)
				return false, err
			}
			registryIngress := deploy.NewIngress(instance, registryName, registryName, 8080)

			if err := r.CreateNewIngress(instance, registryIngress); err != nil {
				logrus.Errorf("Failed to create %s %s: %s", registryIngress.Name, registryIngress.Kind, err)
				return false, err
			}
			return true, nil
		}

		currentRegistryRoute := r.GetEffectiveRoute(instance, registryName)
		if currentRegistryRoute == nil {
			return false, err
		}
		logrus.Infof("Deleting route %s", currentRegistryRoute.Name)
		if err := r.client.Delete(context.TODO(), currentRegistryRoute); err != nil {
			logrus.Errorf("Failed to delete %s route: %s", currentRegistryRoute.Name, err)
			return false, err
		}
		registryRoute := deploy.NewRoute(instance, registryName, registryName, 8080)

		if tlsSupport {
			registryRoute = deploy.NewTlsRoute(instance, registryName, registryName, 8080)
		}

		if err := r.CreateNewRoute(instance, registryRoute); err != nil {
			logrus.Errorf("Failed to create %s %s: %s", registryRoute.Name, registryRoute.Kind, err)
			return false, err
		}
		return true, nil
	}

	updated, err = updateRegistryRoute("devfile")
	if !(updated || instance.Spec.Server.ExternalDevfileRegistry) || err != nil {
		return updated, err
	}

	updated, err = updateRegistryRoute("plugin")
	if !(updated || instance.Spec.Server.ExternalPluginRegistry) || err != nil {
		return updated, err
	}

	protocol := "http"
	if tlsSupport {
		protocol = "https"
	}
	// reconcile ingresses
	if !isOpenShift {
		ingressDomain := instance.Spec.K8s.IngressDomain
		ingressStrategy := util.GetValue(instance.Spec.K8s.IngressStrategy, deploy.DefaultIngressStrategy)
		currentCheIngress := r.GetEffectiveIngress(instance, cheFlavor)
		if currentCheIngress == nil {
			return false, err
		}
		logrus.Infof("Deleting ingress %s", currentCheIngress.Name)
		if err := r.client.Delete(context.TODO(), currentCheIngress); err != nil {
			logrus.Errorf("Failed to delete %s ingress: %s", currentCheIngress.Name, err)
			return false, err
		}
		cheIngress := deploy.NewIngress(instance, cheFlavor, "che-host", 8080)

		if err := r.CreateNewIngress(instance, cheIngress); err != nil {
			logrus.Errorf("Failed to create %s %s: %s", cheIngress.Name, cheIngress.Kind, err)
			return false, err
		}
		currentKeycloakIngress := r.GetEffectiveIngress(instance, "keycloak")
		if currentKeycloakIngress == nil {
			return false, err
		} else {
			keycloakURL := protocol + "://" + ingressDomain
			if ingressStrategy == "multi-host" {
				keycloakURL = protocol + "://keycloak-" + instance.Namespace + "." + ingressDomain
			}
			instance.Spec.Auth.IdentityProviderURL = keycloakURL
			if err := r.UpdateCheCRSpec(instance, "Keycloak URL", keycloakURL); err != nil {
				return false, err
			}
		}
		logrus.Infof("Deleting ingress %s", currentKeycloakIngress.Name)
		if err := r.client.Delete(context.TODO(), currentKeycloakIngress); err != nil {
			logrus.Errorf("Failed to delete %s ingress: %s", currentKeycloakIngress.Name, err)
			return false, err
		}
		keycloakIngress := deploy.NewIngress(instance, "keycloak", "keycloak", 8080)

		if err := r.CreateNewIngress(instance, keycloakIngress); err != nil {
			logrus.Errorf("Failed to create Keycloak ingress: %s", err)
			return false, err
		}
		return true, nil

	}
	currentCheRoute := r.GetEffectiveRoute(instance, cheFlavor)
	if currentCheRoute == nil {
		return false, err

	}
	logrus.Infof("Deleting route %s", currentCheRoute.Name)
	if err := r.client.Delete(context.TODO(), currentCheRoute); err != nil {
		logrus.Errorf("Failed to delete %s route: %s", currentCheRoute.Name, err)
		return false, err
	}
	cheRoute := deploy.NewRoute(instance, cheFlavor, "che-host", 8080)

	if tlsSupport {
		cheRoute = deploy.NewTlsRoute(instance, cheFlavor, "che-host", 8080)
	}

	if err := r.CreateNewRoute(instance, cheRoute); err != nil {
		logrus.Errorf("Failed to create %s %s: %s", cheRoute.Name, cheRoute.Kind, err)
		return false, err
	}

	currentKeycloakRoute := &routev1.Route{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "keycloak", Namespace: instance.Namespace}, currentKeycloakRoute); err != nil {
		logrus.Errorf("Failed to get %s route: %s", currentKeycloakRoute.Name, err)
		return false, err

	} else {
		keycloakURL := currentKeycloakRoute.Spec.Host
		instance.Spec.Auth.IdentityProviderURL = protocol + "://" + keycloakURL
		if err := r.UpdateCheCRSpec(instance, "Keycloak URL", protocol+"://"+keycloakURL); err != nil {
			return false, err
		}
	}

	logrus.Infof("Deleting route %s", currentKeycloakRoute.Name)
	if err := r.client.Delete(context.TODO(), currentKeycloakRoute); err != nil {
		logrus.Errorf("Failed to delete %s route: %s", currentKeycloakRoute.Name, err)
		return false, err
	}
	keycloakRoute := deploy.NewRoute(instance, "keycloak", "keycloak", 8080)

	if tlsSupport {
		keycloakRoute = deploy.NewTlsRoute(instance, "keycloak", "keycloak", 8080)
	}
	if err := r.CreateNewRoute(instance, keycloakRoute); err != nil {
		logrus.Errorf("Failed to create Keycloak route: %s", err)
		return false, err
	}
	return true, nil
}

func (r *ReconcileChe) ReconcileIdentityProvider(instance *orgv1.CheCluster, isOpenShift4 bool) (deleted bool, err error) {
	if instance.Spec.Auth.OpenShiftoAuth == false && instance.Status.OpenShiftoAuthProvisioned == true {
		keycloakAdminPassword := instance.Spec.Auth.IdentityProviderPassword
		keycloakDeployment := &appsv1.Deployment{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "keycloak", Namespace: instance.Namespace}, keycloakDeployment); err != nil {
			logrus.Errorf("Deployment %s not found: %s", keycloakDeployment.Name, err)
		}
		deleteOpenShiftIdentityProviderProvisionCommand := deploy.GetDeleteOpenShiftIdentityProviderProvisionCommand(instance, keycloakAdminPassword, isOpenShift4)
		podToExec, err := k8sclient.GetDeploymentPod(keycloakDeployment.Name, instance.Namespace)
		if err != nil {
			logrus.Errorf("Failed to retrieve pod name. Further exec will fail")
		}
		provisioned := ExecIntoPod(podToExec, deleteOpenShiftIdentityProviderProvisionCommand, "delete OpenShift identity provider", instance.Namespace)
		if provisioned {
			oAuthClient := &oauth.OAuthClient{}
			oAuthClientName := instance.Spec.Auth.OAuthClientName
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName, Namespace: ""}, oAuthClient); err != nil {
				logrus.Errorf("OAuthClient %s not found: %s", oAuthClient.Name, err)
			}
			if err := r.client.Delete(context.TODO(), oAuthClient); err != nil {
				logrus.Errorf("Failed to delete %s %s: %s", oAuthClient.Kind, oAuthClient.Name, err)
			}
			return true, nil
		}
		return false, err
	}
	return false, nil
}
