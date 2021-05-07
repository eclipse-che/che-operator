#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

# Link ocp account with Keycloak IDP
function provisionOAuth() {
  OCP_USER_UID=$(oc get user user -o=jsonpath='{.metadata.uid}')

  IDP_USERNAME="admin"
  CHE_USERNAME="admin"
  # Get Eclipse Che IDP secrets and decode to use to connect to IDP
  IDP_PASSWORD=$(oc get secret che-identity-secret -n eclipse-che -o=jsonpath='{.data.password}' | base64 --decode)

  # Get Auth Route
  if [[ "${CHE_EXPOSURE_STRATEGY}" == "single-host" ]]; then
    IDP_HOST="https://"$(oc get route che -n eclipse-che -o=jsonpath='{.spec.host}')
  fi

  if [[ "${CHE_EXPOSURE_STRATEGY}" == "multi-host" ]]; then
    IDP_HOST="https://"$(oc get route keycloak -n eclipse-che -o=jsonpath='{.spec.host}')
  fi

  # Get the oauth client from Eclipse Che Custom Resource
  OAUTH_CLIENT_NAME=$(oc get checluster eclipse-che -n eclipse-che -o=jsonpath='{.spec.auth.oAuthClientName}')

  # Obtain from Keycloak the token to make api request authentication
  IDP_TOKEN=$(curl -k --location --request POST ''$IDP_HOST'/auth/realms/master/protocol/openid-connect/token' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'username='$IDP_USERNAME'' \
  --data-urlencode 'password='$IDP_PASSWORD'' \
  --data-urlencode 'grant_type=password' \
  --data-urlencode 'client_id=admin-cli' | jq -r .access_token)

  echo -e "[INFO] IDP Token: $IDP_TOKEN"

  # Get admin user id from IDP
  CHE_USER_ID=$(curl --location -k --request GET ''$IDP_HOST'/auth/admin/realms/che/users' \
  --header 'Authorization: Bearer '$IDP_TOKEN'' | jq -r '.[] | select(.username == "'$CHE_USERNAME'").id' )

  echo -e "[INFO] Eclipse CHE user ID: $CHE_USER_ID"

  # Request to link Openshift user with Identity Provider user. In this case we are linked an existed user in IDP
  curl --location -k --request POST ''$IDP_HOST'/auth/admin/realms/che/users/'$CHE_USER_ID'/federated-identity/openshift-v4' \
  --header 'Authorization: Bearer '$IDP_TOKEN'' \
  --header 'Content-Type: application/json' \
  --data '{
      "identityProvider": "openshift-v4",
      "userId": "'$OCP_USER_UID'",
      "userName": "'$CHE_USERNAME'"
  }'

# Create OAuthClientAuthorization object for Eclipse Che in Cluster.
OAUTHCLIENTAuthorization=$(
    oc create -f - -o jsonpath='{.metadata.name}' <<EOF
apiVersion: oauth.openshift.io/v1
kind: OAuthClientAuthorization
metadata:
  generateName: $CHE_USERNAME:$OAUTH_CLIENT_NAME
  namespace: eclipse-che
clientName: $OAUTH_CLIENT_NAME
userName: $CHE_USERNAME
userUID: $OCP_USER_UID
scopes:
  - 'user:full'
EOF
)
  # Create SQL script
  echo -e "Created authorization client: $OAUTHCLIENTAuthorization"
  cat << 'EOF' > path.sql
UPDATE federated_identity SET token ='{"access_token":"INSERT_TOKEN_HERE","expires_in":86400,"scope":"user:full","token_type":"Bearer"}'
WHERE federated_username = 'INSERT_CHE_USER_HERE'
EOF

  TOKEN=$(oc whoami -t)
  sed -i "s|INSERT_TOKEN_HERE|$TOKEN|g" path.sql
  sed -i "s|INSERT_CHE_USER_HERE|$CHE_USERNAME|g" path.sql

  # Insert sql script inside of postgres and execute it.
  POSTGRES_POD=$(oc get pods -o json -n eclipse-che | jq -r '.items[] | select(.metadata.name | test("postgres-")).metadata.name')
  oc cp path.sql "${POSTGRES_POD}":/tmp/ -n eclipse-che
  oc exec -it "${POSTGRES_POD}" -n eclipse-che  -- bash -c "psql -U postgres -d keycloak -d keycloak -f /tmp/path.sql"

  rm path.sql
}
