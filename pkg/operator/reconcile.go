//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"time"
)

// ReconcileChe creates and watches k8s and OpenShift objects
// This is where the operator logic happens
var StartTime = time.Now()

func ReconcileChe() {
	infra := util.GetInfra()
	k8s := GetK8SConfig()
	// service accounts, roles and role-binding for Che
	CreateServiceAccount("che")
	CreateServiceAccount("che-workspace")
	CreateNewRole("exec", []string{"pods/exec"}, []string{"create"})
	CreateNewRole("view", []string{"pods"}, []string{"list"})
	CreateRoleBinding("RoleBinding", "che", "che", "edit", "ClusterRole")
	CreateRoleBinding("RoleBinding", "che-workspace-exec", "che-workspace", "exec", "Role")
	CreateRoleBinding("RoleBinding", "che-workspace-view", "che-workspace", "view", "Role")

	if len(selfSignedCert) > 0 {
		CreateCertSecret()
	}

	// Create and watch Postgres related resources, unless external DB hostname is provided
	if ! externalDb {
		CreatePVC("postgres-data", "1Gi", postgresLabels)
		CreateService("postgres", postgresLabels, "postgres", 5432)
		CreatePostgresDeployment()
		// provision 2 databases, 1 user, and grant superuser privileges to Che pg user
		ExecIntoPod(k8s.GetDeploymentPod("postgres"), pgCommand)
	}

	// Che related envs required for Keycloak
	CreateService("che-host", cheLabels, "http", 8080)

	if tlsSupport {
		protocol = "https"
		wsprotocol = "wss"
	}
	// create Che route when on OpenShift infra
	if infra == "openshift" {
		rt, _ := CreateRouteIfNotExists(cheFlavor, "che-host")
		cheHost = rt.Spec.Host
	} else {
		// create Che ingress when on k8s infra
		CreateIngressIfNotExists(cheFlavor, "che-host", 8080)
		cheHost = ingressDomain
		if strategy == "multi-host" {
			cheHost = cheFlavor + "-" + namespace + "." + ingressDomain
		}
	}

	// Create and watch Keycloak resources
	if ! externalKeycloak {
		CreateService("keycloak", keycloakLabels, "keycloak", 8080)

		if infra == "openshift" {
			keycloakRoute, _ := CreateRouteIfNotExists("keycloak", "keycloak")
			keycloakURL = protocol + "://" + keycloakRoute.Spec.Host
		} else {
			CreateIngressIfNotExists("keycloak", "keycloak", 8080)
			keycloakURL = protocol + "://" + ingressDomain
			if strategy == "multi-host" {
				keycloakURL = protocol + "://" + "keycloak-" + namespace + "." + ingressDomain
			}
		}
		keycloakDc, _ := CreateKeycloakDeployment()
		err := sdk.Update(keycloakDc)
		if err != nil {
			logrus.Errorf("Failed to update Keycloak deployment : %v", err)
		}
		// provision Keycloak realm, client and user
		ExecIntoPod(k8s.GetDeploymentPod("keycloak"), GetKeycloakProvisionCommand(keycloakURL, cheHost))
	}

	if openshiftOAuth {
		CreateOAuthClient(oAuthClientName, oauthSecret, keycloakURL, keycloakRealm)
	}

	// Create and watch Che server resources
	cheConfigMap := CreateCheConfigMap(cheHost, keycloakURL)
	err := sdk.Update(cheConfigMap)
	if err != nil {
		logrus.Errorf("Failed to update Che configmap : %v", err)
	}
	if cheFlavor == "codeready" {
		cheImage = util.GetEnv("CHE_IMAGE", "registry.access.redhat.com/codeready-workspaces-beta/server:latest")
	}
	cheDc, _ := CreateCheDeployment(cheImage)
	err = sdk.Update(cheDc)
	if err != nil {
		logrus.Errorf("Failed to update Che deployment : %v", err)
	}
}
