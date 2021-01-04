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
	"errors"
	"io/ioutil"
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

func GetKeycloakProvisionCommand(cr *v1.CheCluster) (command string, err error) {
	cheFlavor := deploy.DefaultCheFlavor(cr)
	requiredActions := (map[bool]string{true: "\"UPDATE_PASSWORD\"", false: ""})[cr.Spec.Auth.UpdateAdminPassword]
	keycloakTheme := (map[bool]string{true: "rh-sso", false: "che"})[cheFlavor == "codeready"]
	realmDisplayName := (map[bool]string{true: "CodeReady Workspaces", false: "Eclipse Che"})[cheFlavor == "codeready"]

	script, keycloakRealm, keycloakClientId, keycloakUserEnvVar, keycloakPasswordEnvVar := getDefaults(cr)
	data := struct {
		Script                string
		KeycloakAdminUserName string
		KeycloakAdminPassword string
		KeycloakRealm         string
		RealmDisplayName      string
		KeycloakTheme         string
		CheHost               string
		KeycloakClientId      string
		RequiredActions       string
	}{
		script,
		keycloakUserEnvVar,
		keycloakPasswordEnvVar,
		keycloakRealm,
		realmDisplayName,
		keycloakTheme,
		cr.Spec.Server.CheHost,
		keycloakClientId,
		requiredActions,
	}
	return getCommandFromTemplateFile(cr, "/tmp/keycloak-provision.sh", data)
}

func GetOpenShiftIdentityProviderProvisionCommand(cr *v1.CheCluster, oAuthClientName string, oauthSecret string) (string, error) {
	isOpenShift4 := util.IsOpenShift4
	providerId := (map[bool]string{true: "openshift-v4", false: "openshift-v3"})[isOpenShift4]
	openShiftApiUrl, err := util.GetClusterPublicHostname(isOpenShift4)
	if err != nil {
		logrus.Errorf("Failed to auto-detect public OpenShift API URL. Configure it in Identity provider details page in Keycloak admin console: %s", err)
		return "", err
	}

	script, keycloakRealm, keycloakClientId, keycloakUserEnvVar, keycloakPasswordEnvVar := getDefaults(cr)
	data := struct {
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
	}
	return getCommandFromTemplateFile(cr, "/tmp/oauth-provision.sh", data)
}

func GetGitHubIdentityProviderProvisionCommand(deployContext *deploy.DeployContext) (string, error) {
	cr := deployContext.CheCluster
	secretName := cr.Spec.Auth.FederatedIdentities.GitHub.CredentialsSecret
	if secretName == "" {
		return "", errors.New("GitHub credentials secret is empty")
	}

	secret, err := deploy.GetClusterSecret(secretName, cr.Namespace, deployContext.ClusterAPI)
	if err != nil {
		return "", err
	} else if secret == nil {
		return "", errors.New("GitHub credentials secret '" + secretName + "' not found.")
	}

	githubClientId := string(secret.Data["clientId"])
	githuhClientSecret := string(secret.Data["clientSecret"])
	script, keycloakRealm, _, keycloakUserEnvVar, keycloakPasswordEnvVar := getDefaults(cr)
	data := struct {
		Script                string
		KeycloakAdminUserName string
		KeycloakAdminPassword string
		KeycloakRealm         string
		ProviderId            string
		GithubClientId        string
		GithubClientSecret    string
	}{
		script,
		keycloakUserEnvVar,
		keycloakPasswordEnvVar,
		keycloakRealm,
		"github",
		githubClientId,
		githuhClientSecret,
	}
	return getCommandFromTemplateFile(cr, "/tmp/create-github-identity-provider.sh", data)
}

func GetDeleteIdentityProviderCommand(cr *v1.CheCluster, identityProvider string) (string, error) {
	script, keycloakRealm, _, keycloakUserEnvVar, keycloakPasswordEnvVar := getDefaults(cr)
	data := struct {
		Script                string
		KeycloakRealm         string
		KeycloakAdminUserName string
		KeycloakAdminPassword string
		ProviderId            string
	}{
		script,
		keycloakRealm,
		keycloakUserEnvVar,
		keycloakPasswordEnvVar,
		identityProvider,
	}
	return getCommandFromTemplateFile(cr, "/tmp/delete-identity-provider.sh", data)
}

func getCommandFromTemplateFile(cr *v1.CheCluster, templateFile string, data interface{}) (string, error) {
	cheFlavor := deploy.DefaultCheFlavor(cr)

	file, err := ioutil.ReadFile(templateFile)
	if err != nil {
		return "", err
	}

	template, err := template.New("Template").Parse(string(file))
	if err != nil {
		return "", err
	}

	buffer := new(bytes.Buffer)
	err = template.Execute(buffer, data)
	if err != nil {
		return "", err
	}

	command := buffer.String()
	if cheFlavor == "che" {
		command = "cd /scripts && export JAVA_TOOL_OPTIONS=-Duser.home=. && " + command
	}
	return command, nil
}

func getDefaults(cr *v1.CheCluster) (string, string, string, string, string) {
	cheFlavor := deploy.DefaultCheFlavor(cr)
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(cr.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	if cheFlavor == "codeready" {
		return "/opt/eap/bin/kcadm.sh", keycloakRealm, keycloakClientId, "${SSO_ADMIN_USERNAME}", "${SSO_ADMIN_PASSWORD}"
	}

	return "/opt/jboss/keycloak/bin/kcadm.sh", keycloakRealm, keycloakClientId, "${KEYCLOAK_USER}", "${KEYCLOAK_PASSWORD}"
}
