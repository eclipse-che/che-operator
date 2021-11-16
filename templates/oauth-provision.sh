#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

connect_to_keycloak() {
  {{ .Script }} config credentials --server http://0.0.0.0:8080/auth --realm master --user {{ .KeycloakAdminUserName }} --password {{ .KeycloakAdminPassword }}
  {{ .Script }} config truststore --trustpass ${SSO_TRUSTSTORE_PASSWORD} ${SSO_TRUSTSTORE_DIR}/${SSO_TRUSTSTORE}
}

create_identity_provider() {
  {{ .Script }} get identity-provider/instances/{{ .ProviderId }} -r {{ .KeycloakRealm }}
  if [ $? -eq 0 ]
  then echo "Provider exists"
  else {{ .Script }} create identity-provider/instances -r {{ .KeycloakRealm }} -s alias={{ .ProviderId }} -s providerId={{ .ProviderId }} -s enabled=true -s storeToken=true -s addReadTokenRoleOnCreate=true -s config.useJwksUrl=true -s config.clientId={{ .OAuthClientName}} -s config.clientSecret={{ .OauthSecret}} -s config.baseUrl={{ .OpenShiftApiUrl }} -s config.defaultScope=user:full
  fi
}

default_to_openshift_login() {
  EXECUTION_ID=$({{ .Script }} get authentication/flows/browser/executions -r {{ .KeycloakRealm }} -c | sed -e 's/.*\({[^}]\+"identity-provider-redirector"[^}]\+}\).*/\1/' -e 's/.*"id":"\([^"]\+\)".*/\1/')
  ALIAS=$({{ .Script }} get authentication/flows/browser/executions -r {{ .KeycloakRealm }} -c | sed -e 's/.*\({[^}]\+"identity-provider-redirector"[^}]\+}\).*/\1/' | grep '"alias":"' | sed -e 's/.*"alias":"\([^"]\+\)".*/\1/')
  if [ "${EXECUTION_ID}" == "" ]
  then
    echo "Could not find the identity provider redirector"
    return 1
  fi
  if [ -z ${ALIAS} ];
  then
    echo '{"config":{"defaultProvider":"{{ .ProviderId }}"},"alias":"{{ .ProviderId }}"}' | {{ .Script }} create -r {{ .KeycloakRealm }} authentication/executions/${EXECUTION_ID}/config -f -
  fi
}

enable_openshift_token-exchange() {
  IDENTITY_PROVIDER_ID=$({{ .Script }} get -r {{ .KeycloakRealm }} identity-provider/instances/{{ .ProviderId }} | grep -e '"internalId" *: *"' | sed -e 's/.*"internalId" *: *"\([^"]\+\)".*/\1/')
  if [ "${IDENTITY_PROVIDER_ID}" == "" ]
  then
    echo "identity provider not found"
    return 1
  fi
  echo '{"enabled": true}' | {{ .Script }} update -r {{ .KeycloakRealm }} identity-provider/instances/{{ .ProviderId }}/management/permissions -f -
  if [ $? -ne 0 ]
  then
    echo "failed to enable permissions on identity provider"
    return 1
  fi
  TOKEN_EXCHANGE_PERMISSIONS=$({{ .Script }} get -r {{ .KeycloakRealm }} identity-provider/instances/{{ .ProviderId }}/management/permissions)
  if [ "${TOKEN_EXCHANGE_PERMISSIONS}" == "" ]
  then
    echo "token exchange permissions not found"
    return 1
  fi
  TOKEN_EXCHANGE_RESOURCE=$(echo ${TOKEN_EXCHANGE_PERMISSIONS} | grep -e '"resource" *: *"' | sed -e 's/.*"resource" *: *"\([^"]\+\)".*/\1/')
  TOKEN_EXCHANGE_PERMISSION_ID=$(echo ${TOKEN_EXCHANGE_PERMISSIONS} | sed -e 's/.*"scopePermissions" *: *{ *"token-exchange" *: *"\([^"]\+\)".*/\1/')
  if [ "${TOKEN_EXCHANGE_RESOURCE}" == "" ] || [ "${TOKEN_EXCHANGE_PERMISSION_ID}" == "" ]
  then
    echo "token exchange permissions do not contain expected values"
    return 1
  fi
  REALM_MGMT_CLIENT_ID=$({{ .Script }} get -r {{ .KeycloakRealm }} clients -q clientId=realm-management | grep -e '"id" *: *"' | sed -e 's/.*"id" *: *"\([^"]\+\)".*/\1/')
  if [ "${REALM_MGMT_CLIENT_ID}" == "" ]
  then
    echo "Realm management client ID not found"
    return 1
  fi
  EXISTING_POLICY=$({{ .Script }} get -r {{ .KeycloakRealm }} clients/${REALM_MGMT_CLIENT_ID}/authz/resource-server/policy/client -q 'name={{ .ProviderId }}' | grep -e '"id" *: *"' | sed -e 's/.*"id" *: *"\([^"]\+\)".*/\1/')
  if [ "${EXISTING_POLICY}" == "" ]
  then
    echo '{"type":"client","logic":"POSITIVE","decisionStrategy":"UNANIMOUS","name":"{{ .ProviderId }}","clients":["{{ .KeycloakClientId }}"]}' | {{ .Script }} create -r {{ .KeycloakRealm }} clients/${REALM_MGMT_CLIENT_ID}/authz/resource-server/policy/client -f -
    if [ $? -ne 0 ]
    then
      echo "Failed to create policy"
      return 1
    fi
  fi
  TOKEN_EXCHANGE_POLICY=$({{ .Script }} get -r {{ .KeycloakRealm }} clients/${REALM_MGMT_CLIENT_ID}/authz/resource-server/policy/client -q 'name={{ .ProviderId }}' | grep -e '"id" *: *"' | sed -e 's/.*"id" *: *"\([^"]\+\)".*/\1/')
  if [ "${TOKEN_EXCHANGE_POLICY}" == "" ]
  then
    echo "Token exchange policy not found"
    return 1
  fi
  TOKEN_EXCHANGE_PERMISSION=$({{ .Script }} get -r {{ .KeycloakRealm }} clients/${REALM_MGMT_CLIENT_ID}/authz/resource-server/permission/scope/${TOKEN_EXCHANGE_PERMISSION_ID})
  if [ "${TOKEN_EXCHANGE_PERMISSION}" == "" ]
  then
    echo "Token exchange permission not found"
    return 1
  fi
  TOKEN_EXCHANGE_SCOPES=$({{ .Script }} get -r {{ .KeycloakRealm }} clients/${REALM_MGMT_CLIENT_ID}/authz/resource-server/resource/${TOKEN_EXCHANGE_RESOURCE}/scopes)
  if [ "${TOKEN_EXCHANGE_SCOPES}" == "" ]
  then
    echo "Token exchange scopes not found"
    return 1
  fi
  TOKEN_EXCHANGE_SCOPE=$(echo ${TOKEN_EXCHANGE_SCOPES} | sed -e 's/.*"id" *: *"\([^"]\+\)" *, *"name" *: *"token-exchange".*/\1/')
  if [ "${TOKEN_EXCHANGE_SCOPE}" == "" ]
  then
    echo "Token exchange scope not found"
    return 1
  fi
  PERMISSION_RESOURCES=$(echo ${TOKEN_EXCHANGE_PERMISSION} | grep -e 'resources *:')
  if [ "${PERMISSION_RESOURCES}" == "" ]
  then
    echo ${TOKEN_EXCHANGE_PERMISSION} | sed -e "s/ *{\(.*}\) */{\"resources\":[\"${TOKEN_EXCHANGE_RESOURCE}\"],\"scopes\":[\"${TOKEN_EXCHANGE_SCOPE}\"],\"policies\":[\"${TOKEN_EXCHANGE_POLICY}\"],\1/" | {{ .Script }} update -r {{ .KeycloakRealm }} clients/${REALM_MGMT_CLIENT_ID}/authz/resource-server/permission/scope/${TOKEN_EXCHANGE_PERMISSION_ID} -f -
  fi
}

set -x
connect_to_keycloak && create_identity_provider && default_to_openshift_login && enable_openshift_token-exchange
