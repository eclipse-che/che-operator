#
# Copyright (c) 2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

connectToKeycloak() {
  {{ .Script }} config credentials --server http://0.0.0.0:8080/auth --realm master --user {{ .KeycloakAdminUserName }} --password {{ .KeycloakAdminPassword }}
}

provisionKeycloak() {
  {{ .Script }} update realms/master -s sslRequired=none
  {{ .Script }} config truststore --trustpass ${SSO_TRUSTSTORE_PASSWORD} ${SSO_TRUSTSTORE_DIR}/${SSO_TRUSTSTORE}

  {{ .Script }} get realms/{{ .KeycloakRealm }}
  if [ $? -eq 0 ]; then
    echo "{{ .KeycloakRealm }} realm exists."
    exit 0
  fi

  echo "Provision {{ .KeycloakRealm }} realm."
  {{ .Script }} create realms  \
    -s realm='{{ .KeycloakRealm }}' \
    -s displayName='{{ .RealmDisplayName }}' \
    -s enabled=true \
    -s sslRequired=none \
    -s registrationAllowed=false \
    -s resetPasswordAllowed=true \
    -s loginTheme={{ .KeycloakTheme }} \
    -s accountTheme={{ .KeycloakTheme }} \
    -s adminTheme={{ .KeycloakTheme }} \
    -s emailTheme={{ .KeycloakTheme }}

  {{ .Script }} create clients \
    -r '{{ .KeycloakRealm }}' \
    -s clientId={{ .KeycloakClientId }} \
    -s id={{ .KeycloakClientId }} \
    -s webOrigins='["http://{{ .CheHost }}", "https://{{ .CheHost }}"]' \
    -s redirectUris='["http://{{ .CheHost }}/dashboard/*", "https://{{ .CheHost }}/dashboard/*", "http://{{ .CheHost }}/factory*", "https://{{ .CheHost }}/factory*", "http://{{ .CheHost }}/f*", "https://{{ .CheHost }}/f*", "http://{{ .CheHost }}/_app/*", "https://{{ .CheHost }}/_app/*", "http://{{ .CheHost }}/swagger/*", "https://{{ .CheHost }}/swagger/*"]' \
    -s directAccessGrantsEnabled=true \
    -s publicClient=true

  {{ .Script }} create users \
    -r '{{ .KeycloakRealm }}' \
    -s username=admin \
    -s email=\"admin@admin.com\" \
    -s enabled=true \
    -s requiredActions='[{{ .RequiredActions }}]'

  {{ .Script }} set-password \
    -r '{{ .KeycloakRealm }}' \
    --username admin \
    --new-password admin

  {{ .Script }} add-roles \
    -r '{{ .KeycloakRealm }}' \
    --uusername admin \
    --cclientid broker \
    --rolename read-token

  CLIENT_ID=$({{ .Script }} get clients -r '{{ .KeycloakRealm }}' -q clientId=broker | sed -n 's/.*"id" *: *"\([^"]\+\).*/\1/p')
  {{ .Script }} update clients/${CLIENT_ID} \
    -r '{{ .KeycloakRealm }}' \
    -s "defaultRoles+=read-token"
}

connectToKeycloak
provisionKeycloak
