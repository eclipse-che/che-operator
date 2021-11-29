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

connectToKeycloak() {
  {{ .Script }} config credentials --server http://0.0.0.0:8080/auth --realm master --user {{ .KeycloakAdminUserName }} --password {{ .KeycloakAdminPassword }}
}

createIdentityProvider() {
  {{ .Script }} get identity-provider/instances/{{ .ProviderId }} -r {{ .KeycloakRealm }}
  if [ $? -eq 0 ]; then
    echo "{{ .ProviderId }} identity provider exists."
  else
    echo "Create new {{ .ProviderId }} identity provider."
    if [ -z "${GITHUB_CLIENT_ID}" ] || [ -z "${GITHUB_SECRET}" ]; then
      echo "Either 'GITHUB_CLIENT_ID' or 'GITHUB_SECRET' aren't set" 1>&2
      exit 1
    fi
    {{ .Script }} create identity-provider/instances \
    -r {{ .KeycloakRealm }} \
    -s alias={{ .ProviderId }} \
    -s providerId={{ .ProviderId }} \
    -s enabled=true \
    -s storeToken=true \
    -s config.useJwksUrl=true \
    -s config.clientId=${GITHUB_CLIENT_ID} \
    -s config.clientSecret=${GITHUB_SECRET} \
    -s config.defaultScope=repo,user,write:public_key
  fi
}

connectToKeycloak
createIdentityProvider
