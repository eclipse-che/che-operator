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
	"bytes"
	"io/ioutil"
	"strings"
	"text/template"

	v1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

func GetPostgresProvisionCommand(identityProviderPostgresPassword string) (command string) {
	command = "OUT=$(psql postgres -tAc \"SELECT 1 FROM pg_roles WHERE rolname='keycloak'\"); " +
		"if [ $OUT -eq 1 ]; then echo \"DB exists\"; exit 0; fi " +
		"&& psql -c \"CREATE USER keycloak WITH PASSWORD '" + identityProviderPostgresPassword + "'\" " +
		"&& psql -c \"CREATE DATABASE keycloak\" " +
		"&& psql -c \"GRANT ALL PRIVILEGES ON DATABASE keycloak TO keycloak\" " +
		"&& psql -c \"ALTER USER ${POSTGRESQL_USER} WITH SUPERUSER\""

	return command
}

func GetKeycloakProvisionCommand(cr *v1.CheCluster) (command string) {
	requiredActions := ""
	updateAdminPassword := cr.Spec.Auth.UpdateAdminPassword
	cheFlavor := deploy.DefaultCheFlavor(cr)
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(cr.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	keycloakUserEnvVar := "${KEYCLOAK_USER}"
	keycloakPasswordEnvVar := "${KEYCLOAK_PASSWORD}"

	if updateAdminPassword {
		requiredActions = "\"UPDATE_PASSWORD\""
	}
	file, err := ioutil.ReadFile("/tmp/keycloak_provision")
	if err != nil {
		logrus.Errorf("Failed to locate keycloak entrypoint file: %s", err)
	}
	keycloakTheme := "che"
	realmDisplayName := "Eclipse Che"
	script := "/opt/jboss/keycloak/bin/kcadm.sh"
	if cheFlavor == "codeready" {
		keycloakTheme = "rh-sso"
		realmDisplayName = "CodeReady Workspaces"
		script = "/opt/eap/bin/kcadm.sh"
		keycloakUserEnvVar = "${SSO_ADMIN_USERNAME}"
		keycloakPasswordEnvVar = "${SSO_ADMIN_PASSWORD}"
	}
	str := string(file)
	r := strings.NewReplacer("$script", script,
		"$keycloakAdminUserName", keycloakUserEnvVar,
		"$keycloakAdminPassword", keycloakPasswordEnvVar,
		"$keycloakRealm", keycloakRealm,
		"$realmDisplayName", realmDisplayName,
		"$keycloakClientId", keycloakClientId,
		"$keycloakTheme", keycloakTheme,
		"$cheHost", cr.Spec.Server.CheHost,
		"$requiredActions", requiredActions)
	createRealmClientUserCommand := r.Replace(str)
	command = createRealmClientUserCommand
	if cheFlavor == "che" {
		command = "cd /scripts && export JAVA_TOOL_OPTIONS=-Duser.home=. && " + createRealmClientUserCommand
	}
	return command
}

func GetOpenShiftIdentityProviderProvisionCommand(cr *v1.CheCluster, oAuthClientName string, oauthSecret string, isOpenShift4 bool) (command string, err error) {
	cheFlavor := deploy.DefaultCheFlavor(cr)
	openShiftApiUrl, err := util.GetClusterPublicHostname(isOpenShift4)
	if err != nil {
		logrus.Errorf("Failed to auto-detect public OpenShift API URL. Configure it in Identity provider details page in Keycloak admin console: %s", err)
		return "", err
	}

	keycloakUserEnvVar := "${KEYCLOAK_USER}"
	keycloakPasswordEnvVar := "${KEYCLOAK_PASSWORD}"
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	script := "/opt/jboss/keycloak/bin/kcadm.sh"
	if cheFlavor == "codeready" {
		script = "/opt/eap/bin/kcadm.sh"
		keycloakUserEnvVar = "${SSO_ADMIN_USERNAME}"
		keycloakPasswordEnvVar = "${SSO_ADMIN_PASSWORD}"
	}
	keycloakClientId := util.GetValue(cr.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")

	providerId := "openshift-v3"
	if isOpenShift4 {
		providerId = "openshift-v4"
	}

	file, err := ioutil.ReadFile("/tmp/oauth_provision")
	if err != nil {
		logrus.Errorf("Failed to locate keycloak oauth provisioning file: %s", err)
	}
	createOpenShiftIdentityProviderTemplate := string(file)
	/*
		In order to have the token-exchange currently working and easily usable, we should (in case of Keycloak) be able to
		- Automatically redirect the user to its Keycloak account page to set those required values when the email is empty (instead of failing here: https://github.com/eclipse/che/blob/master/multiuser/keycloak/che-multiuser-keycloak-server/src/main/java/org/eclipse/che/multiuser/keycloak/server/KeycloakEnvironmentInitalizationFilter.java#L125)
		- Or at least point with a link to the place where it can be set (the KeycloakSettings PROFILE_ENDPOINT_SETTING value)
		  (cf. here: https://github.com/eclipse/che/blob/master/multiuser/keycloak/che-multiuser-keycloak-server/src/main/java/org/eclipse/che/multiuser/keycloak/server/KeycloakSettings.java#L117)
	*/

	template, err := template.New("IdentityProviderProvisioning").Parse(createOpenShiftIdentityProviderTemplate)
	if err != nil {
		return "", err
	}
	buffer := new(bytes.Buffer)
	err = template.Execute(
		buffer,
		struct {
			Script                string
			KeycloakAdminUserName string
			KeycloakAdminPassword string
			KeycloakRealm         string
			ProviderId            string
			OAuthClientName       string
			OauthSecret           string
			OpenShiftApiUrl       string
			KeycloakClientId      string
		}{
			script,
			keycloakUserEnvVar,
			keycloakPasswordEnvVar,
			keycloakRealm,
			providerId,
			oAuthClientName,
			oauthSecret,
			openShiftApiUrl,
			keycloakClientId,
		})
	if err != nil {
		return "", err
	}

	command = buffer.String()

	if cheFlavor == "che" {
		command = "cd /scripts && export JAVA_TOOL_OPTIONS=-Duser.home=. && " + command
	}
	return command, nil
}

func GetDeleteOpenShiftIdentityProviderProvisionCommand(cr *v1.CheCluster, isOpenShift4 bool) (command string) {
	cheFlavor := deploy.DefaultCheFlavor(cr)
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	script := "/opt/jboss/keycloak/bin/kcadm.sh"
	keycloakUserEnvVar := "${KEYCLOAK_USER}"
	keycloakPasswordEnvVar := "${KEYCLOAK_PASSWORD}"
	if cheFlavor == "codeready" {
		script = "/opt/eap/bin/kcadm.sh"
		keycloakUserEnvVar = "${SSO_ADMIN_USERNAME}"
		keycloakPasswordEnvVar = "${SSO_ADMIN_PASSWORD}"
	}

	providerName := "openshift-v3"
	if isOpenShift4 {
		providerName = "openshift-v4"
	}
	deleteOpenShiftIdentityProviderCommand :=
		script + " config credentials --server http://0.0.0.0:8080/auth " +
			"--realm master --user " + keycloakUserEnvVar + " --password " + keycloakPasswordEnvVar + " && " +
			"if " + script + " get identity-provider/instances/" + providerName + " -r " + keycloakRealm + " ; then " +
			script + " delete identity-provider/instances/" + providerName + " -r " + keycloakRealm + " ; fi"
	command = deleteOpenShiftIdentityProviderCommand
	if cheFlavor == "che" {
		command = "cd /scripts && export JAVA_TOOL_OPTIONS=-Duser.home=. && " + deleteOpenShiftIdentityProviderCommand
	}
	return command
}
