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
package deploy

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"
)

func GetPostgresProvisionCommand(cr *orgv1.CheCluster) (command string) {

	chePostgresUser := util.GetValue(cr.Spec.Database.ChePostgresUser, DefaultChePostgresUser)
	keycloakPostgresPassword := cr.Spec.Auth.KeycloakPostgresPassword

	command = "OUT=$(psql postgres -tAc \"SELECT 1 FROM pg_roles WHERE rolname='keycloak'\"); " +
		"if [ $OUT -eq 1 ]; then echo \"DB exists\"; exit 0; fi " +
		"&& psql -c \"CREATE USER keycloak WITH PASSWORD '" + keycloakPostgresPassword + "'\" " +
		"&& psql -c \"CREATE DATABASE keycloak\" " +
		"&& psql -c \"GRANT ALL PRIVILEGES ON DATABASE keycloak TO keycloak\" " +
		"&& psql -c \"ALTER USER " + chePostgresUser + " WITH SUPERUSER\""

	return command
}

func GetKeycloakProvisionCommand(cr *orgv1.CheCluster, cheHost string) (command string) {
	keycloakAdminUserName := util.GetValue(cr.Spec.Auth.KeycloakAdminUserName,"admin")
	keycloakAdminPassword := util.GetValue(cr.Spec.Auth.KeycloakAdminPassword,"admin")
	requiredActions := ""
	updateAdminPassword := cr.Spec.Auth.UpdateAdminPassword
	cheFlavor := util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor)
	keycloakRealm := util.GetValue(cr.Spec.Auth.KeycloakRealm, cheFlavor)
	keycloakClientId := util.GetValue(cr.Spec.Auth.KeycloakClientId, cheFlavor+"-public")

	if updateAdminPassword {
		requiredActions = "\"UPDATE_PASSWORD\""
	}
	file, err := ioutil.ReadFile("/tmp/keycloak_provision")
	if err != nil {
		logrus.Errorf("Failed to find keycloak entrypoint file %s", err)
	}
	keycloakTheme := "che"
	realmDisplayName := "Eclipse Che"
	script := "/opt/jboss/keycloak/bin/kcadm.sh"
	if cheFlavor == "codeready" {
		keycloakTheme = "rh-sso"
		realmDisplayName = "CodeReady Workspaces"
		script = "/opt/eap/bin/kcadm.sh"

	}
	str := string(file)
	r := strings.NewReplacer("$script", script,
		"$keycloakAdminUserName", keycloakAdminUserName,
		"$keycloakAdminPassword", keycloakAdminPassword,
		"$keycloakRealm", keycloakRealm,
		"$realmDisplayName", realmDisplayName,
		"$keycloakClientId", keycloakClientId,
		"$keycloakTheme", keycloakTheme,
		"$cheHost", cheHost,
		"$requiredActions", requiredActions)
	createRealmClientUserCommand := r.Replace(str)
	command = createRealmClientUserCommand
	if cheFlavor == "che" {
		command = "cd /scripts && " +createRealmClientUserCommand
	}
	return command
}

func GetOpenShiftIdentityProviderProvisionCommand(cr *orgv1.CheCluster, oAuthClientName string, oauthSecret string, keycloakAdminPassword string) (command string) {
	cheFlavor := util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor)
	openShiftApiUrl, err := util.GetClusterPublicHostname()
	if err != nil {
		logrus.Errorf("Failed to auto-detect public OpenShift API URL. Configure it in Identity provider details page in Keycloak admin console: %s", err)
		openShiftApiUrl = "RECPLACE_ME"
	}

	keycloakRealm := util.GetValue(cr.Spec.Auth.KeycloakRealm, cheFlavor)
	keycloakAdminUserName := util.GetValue(cr.Spec.Auth.KeycloakAdminUserName, DefaultKeycloakAdminUserName)
	script := "/opt/jboss/keycloak/bin/kcadm.sh"
	if cheFlavor == "codeready" {
		script = "/opt/eap/bin/kcadm.sh"

	}

	createOpenShiftIdentityProviderCommand :=
		script + " config credentials --server http://0.0.0.0:8080/auth " +
			"--realm master --user " + keycloakAdminUserName + " --password " + keycloakAdminPassword + " && " + script +
			" get identity-provider/instances/openshift-v3 -r " + keycloakRealm + "; " +
			"if [ $? -eq 0 ]; then echo \"Provider exists\"; exit 0; fi && " + script +
			" create identity-provider/instances -r " + keycloakRealm +
			" -s alias=openshift-v3 -s providerId=openshift-v3 -s enabled=true -s storeToken=true" +
			" -s addReadTokenRoleOnCreate=true -s config.useJwksUrl=true" +
			" -s config.clientId=" + oAuthClientName + " -s config.clientSecret=" + oauthSecret +
			" -s config.baseUrl=" + openShiftApiUrl +
			" -s config.defaultScope=user:full"
	command = createOpenShiftIdentityProviderCommand
	if cheFlavor == "che" {
		command = "cd /scripts && " + createOpenShiftIdentityProviderCommand
	}
	return command
}

func GetDeleteOpenShiftIdentityProviderProvisionCommand(cr *orgv1.CheCluster, keycloakAdminPassword string) (command string) {
	cheFlavor := util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor)
	keycloakRealm := util.GetValue(cr.Spec.Auth.KeycloakRealm, cheFlavor)
	keycloakAdminUserName := util.GetValue(cr.Spec.Auth.KeycloakAdminUserName, DefaultKeycloakAdminUserName)
	script := "/opt/jboss/keycloak/bin/kcadm.sh"
	if cheFlavor == "codeready" {
		script = "/opt/eap/bin/kcadm.sh"

	}

	deleteOpenShiftIdentityProviderCommand :=
		script + " config credentials --server http://0.0.0.0:8080/auth " +
			"--realm master --user " + keycloakAdminUserName + " --password " + keycloakAdminPassword + " && " +
			script + " delete identity-provider/instances/openshift-v3 -r " + keycloakRealm
	command = deleteOpenShiftIdentityProviderCommand
	if cheFlavor == "che" {
		command = "cd /scripts && " + deleteOpenShiftIdentityProviderCommand
	}
	return command
}

