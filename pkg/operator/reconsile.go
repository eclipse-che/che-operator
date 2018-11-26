package operator

import (
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"time"
)

// ReconsileChe creates and watches k8s and OpenShift objects
// This is where the operator logic happens
var StartTime = time.Now()

func ReconsileChe() {

	// service accounts, roles and role-binding for Che
	CreateServiceAccount("che")
	CreateServiceAccount("che-workspace")
	CreateNewRole("exec")
	CreateRoleBinding("RoleBinding", "che", "che", "edit", "ClusterRole")
	CreateRoleBinding("RoleBinding", "che-workspace-exec", "che-workspace", "exec", "Role")
	CreateRoleBinding("RoleBinding", "che-workspace-view", "che-workspace", "view", "ClusterRole")

	if len(selfSignedCert) > 0 {
		CreateCertSecret()
	}

	// Create and watch Postgres related resources, unless external DB hostname is provided
	if ! externalDb {
		CreateService( "postgres", postgresLabels, "postgres", 5432)
		CreatePVC( "postgres-data", "1Gi", postgresLabels)
		CreatePostgresDeployment()
		CreatePgJob()
	}

	// Che related envs required for Keycloak
	CreateService( "che-host", cheLabels, "http", 8080)

	if tlsSupport {
		protocol = "https"
		wsprotocol = "wss"
	}
	// create Che route when on OpenShift infra
	if infra == "openshift" {
		rt, _ := CreateRouteIfNotExists( "che", "che-host")
		cheHost = rt.Spec.Host
	} else {
		// create Che ingress when on k8s infra
		CreateIngressIfNotExists( "che", "che-host", 8080)
		cheHost = ingressDomain
		if strategy == "multi-host" {
			cheHost = "che-" + namespace + "." + ingressDomain
		}
	}

	// Create and watch Keycloak resources
	if ! externalKeycloak {
		CreateService( "keycloak", keycloakLabels, "keycloak", 8080)

		if infra == "openshift" {
			keycloakRoute, _ := CreateRouteIfNotExists( "keycloak", "keycloak")
			keycloakURL = protocol + "://" + keycloakRoute.Spec.Host
		} else {
			CreateIngressIfNotExists( "keycloak", "keycloak", 8080)
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
		CreateKeycloakJob(keycloakURL, cheHost)
	}

	if openshiftOAuth {
		CreateOAuthClient(oauthSecret, keycloakURL, keycloakRealm)
	}

	// Create and watch Che server resources
	CreatePVC("che-data-volume", "1Gi", cheLabels)
	CreateCheConfigMap(cheHost, keycloakURL)
	cheDc, _ := CreateCheDeployment(cheImageRepo, cheImageTag)
	err := sdk.Update(cheDc)
	if err != nil {
		logrus.Errorf("Failed to update Che deployment : %v", err)
	}

}
