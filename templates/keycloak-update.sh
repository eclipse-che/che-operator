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

updateKeycloak() {
  DEFAULT_WEBORIGINS='"http://{{ .CheHost }}", "https://{{ .CheHost }}"'
  # ADDITIONAL_WEBORIGINS is an env var in format '"url1", "url2"'
  # which if specified, is provisioned to keycloak additionally to Che's URLs ones
  [ ! -z "$ADDITIONAL_WEBORIGINS" ] && ADDITIONAL_WEBORIGINS=", $ADDITIONAL_WEBORIGINS"
  WEBORIGINS="[$DEFAULT_WEBORIGINS $ADDITIONAL_WEBORIGINS]"

  DEFAULT_REDIRECT_URIS='"http://{{ .CheHost }}/dashboard/*", "https://{{ .CheHost }}/dashboard/*", "http://{{ .CheHost }}/factory*", "https://{{ .CheHost }}/factory*", "http://{{ .CheHost }}/f*", "https://{{ .CheHost }}/f*", "http://{{ .CheHost }}/_app/*", "https://{{ .CheHost }}/_app/*", "http://{{ .CheHost }}/swagger/*", "https://{{ .CheHost }}/swagger/*"'
  # ADDITIONAL_REDIRECT_URIS is an env var in format '"url1", "url2"'
  # which if specified, is provisioned to keycloak additionally to Che's URLs ones
  [ ! -z "$ADDITIONAL_REDIRECT_URIS" ] && ADDITIONAL_REDIRECT_URIS=", $ADDITIONAL_REDIRECT_URIS"
  REDIRECT_URIS="[$DEFAULT_REDIRECT_URIS $ADDITIONAL_REDIRECT_URIS]"

  {{ .Script }} update clients/{{ .KeycloakClientId }} \
    -r '{{ .KeycloakRealm }}' \
    -s webOrigins="$WEBORIGINS" \
    -s redirectUris="$REDIRECT_URIS"
}

checkKeycloak() {
  REDIRECT_URIS=$({{ .Script }} get clients/{{ .KeycloakClientId }} -r '{{ .KeycloakRealm }}' | jq '.redirectUris')
  FIND="http://{{ .CheHost }}/factory*"
  for URI in "${REDIRECT_URIS[@]}"; do
    [[ $FIND == "$URI" ]] && return 0
  done
  return 1
}

connectToKeycloak
checkKeycloak
if [ $? -ne 0 ]
then
  updateKeycloak
fi
