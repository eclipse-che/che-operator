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

deleteIdentityProvider() {
  {{ .Script }} get identity-provider/instances/{{ .ProviderId }} -r {{ .KeycloakRealm }}
  if [ ! $? -eq 0 ]; then
    echo "{{ .ProviderId }} identity provider does not exists."
  else
    echo "Delete {{ .ProviderId }} identity provider."
    {{ .Script }} delete identity-provider/instances/{{ .ProviderId }} -r {{ .KeycloakRealm }}
  fi
}

connectToKeycloak
deleteIdentityProvider
