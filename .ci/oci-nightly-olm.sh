#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

################################ !!!   IMPORTANT   !!! ################################
########### THIS JOB USE openshift ci operators workflows to run  #####################
##########  More info about how it is configured can be found here: https://docs.ci.openshift.org/docs/how-tos/testing-operator-sdk-operators #############
#######################################################################################################################################################

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

overrideDefaults() {
  # CI_CHE_OPERATOR_IMAGE it is che operator image builded in openshift CI job workflow. More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  export OPERATOR_IMAGE=${CI_CHE_OPERATOR_IMAGE:-"quay.io/eclipse/che-operator:nightly"}
  export OAUTH="true"
}

function oauthProvisioned() {
  OCP_USER_UID=$(oc get user admin -o=jsonpath='{.metadata.uid}')

  IDP_USER="admin"
  IDP_PASSWORD=$(oc get secret che-identity-secret -n eclipse-che -o=jsonpath='{.data.password}' | base64 --decode)
  IDP_HOST="https://"$(oc get route keycloak -n eclipse-che -o=jsonpath='{.spec.host}')
  OAUTH_CLIENT_NAME=$(oc get checluster eclipse-che -n eclipse-che -o=jsonpath='{.spec.auth.oAuthClientName}')

  TOKEN_RESULT=$(curl -k --location --request POST ''$IDP_HOST'/auth/realms/master/protocol/openid-connect/token' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'username=admin' \
  --data-urlencode 'password='$IDP_PASSWORD'' \
  --data-urlencode 'grant_type=password' \
  --data-urlencode 'client_id=admin-cli' | jq -r .access_token)

  echo -e "[INFO] Token: $TOKEN_RESULT"

  USER_ID=$(curl --location -k --request GET ''$IDP_HOST'/auth/admin/realms/che/users' \
  --header 'Authorization: Bearer '$TOKEN_RESULT'' | jq -r '.[] | select(.username == "admin").id' )

  echo -e "[INFO] user id: $USER_ID"

  curl --location -k --request POST ''$IDP_HOST'/auth/admin/realms/che/users/'$USER_ID'/federated-identity/openshift-v4' \
  --header 'Authorization: Bearer '$TOKEN_RESULT'' \
  --header 'Content-Type: application/json' \
  --data '{
      "identityProvider": "openshift-v4",
      "userId": "'$OCP_USER_UID'",
      "userName": "admin"
  }'

OAUTHCLIENTAuthorization=$(
    oc create -f - -o jsonpath='{.metadata.name}' <<EOF
apiVersion: oauth.openshift.io/v1
kind: OAuthClientAuthorization
metadata:
  generateName: $IDP_USER:$OAUTH_CLIENT_NAME
  namespace: eclipse-che
clientName: $OAUTH_CLIENT_NAME
userName: $IDP_USER
userUID: $OCP_USER_UID
scopes:
  - 'user:full'
EOF
)

  echo -e "Created authorization client: $OAUTHCLIENTAuthorization"
}

function provisionPostgres() {
cat << 'EOF' > path.sql
UPDATE federated_identity SET token ='{"access_token":"INSERT_TOKEN_HERE","expires_in":86400,"scope":"user:full","token_type":"Bearer"}'
WHERE federated_username = 'admin'
EOF

  TOKEN=$(oc whoami -t)
  sed -i "s|INSERT_TOKEN_HERE|$TOKEN|g" path.sql

  POSTGRES_POD=$(oc get pods -o json -n eclipse-che | jq -r '.items[] | select(.metadata.name | test("postgres-")).metadata.name')

  oc cp path.sql "${POSTGRES_POD}":/tmp/ -n eclipse-che
  oc exec -it "${POSTGRES_POD}" -n eclipse-che  -- bash -c "psql -U postgres -d keycloak -d keycloak -f /tmp/path.sql"

  rm path.sql
}

runTests() {
    # Deploy Eclipse Che applying CR
    applyOlmCR
    waitEclipseCheDeployed "nightly"
    oauthProvisioned
    provisionPostgres
    startNewWorkspace
    waitWorkspaceStart
}

init
provisionOpenshiftUsers
overrideDefaults
patchEclipseCheOperatorSubscription
printOlmCheObjects
runTests
