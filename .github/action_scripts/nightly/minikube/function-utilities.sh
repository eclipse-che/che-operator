#!/usr/bin/env bash
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

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u
# print each command before executing it
set -x

# Build latest operator image
function buildCheOperatorImage() {
    docker build -t "${OPERATOR_IMAGE}" -f Dockerfile . && docker save "${OPERATOR_IMAGE}" > operator.tar
    eval $(minikube docker-env) && docker load -i operator.tar && rm operator.tar
}

# Get Token from single host mode deployment
function getSingleHostToken() {
    export GATEWAY_HOSTNAME=$(minikube ip).nip.io
    export TOKEN_ENDPOINT="https://${GATEWAY_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
    export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
}

# Utility to wait for a workspace to be started after workspace:create.
function waitSingleHostWorkspaceStart() {
  set +e
  export x=0
  while [ $x -le 180 ]
  do
    getSingleHostToken

    # List Workspaces and get the status
    echo "[INFO] Getting workspace status:"
    chectl workspace:list
    workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})
    workspaceStatus=$(echo "$workspaceList" | grep RUNNING | awk '{ print $4} ')

    if [ "${workspaceStatus:-NOT_RUNNING}" == "RUNNING" ]
    then
      echo "[INFO] Workspace started successfully"
      break
    fi
    sleep 10
    x=$(( x+1 ))
  done

  if [ $x -gt 180 ]
  then
    echo "[ERROR] Workspace didn't start after 3 minutes."
    exit 1
  fi
}

# Get the access token from keycloak in openshift platforms and kubernetes
function getCheAcessToken() {
  if [[ ${PLATFORM} == "openshift" ]]
  then
    KEYCLOAK_HOSTNAME=$(oc get route -n ${NAMESPACE} keycloak --template={{.spec.host}})
    TOKEN_ENDPOINT="https://${KEYCLOAK_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
    export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
  else
    KEYCLOAK_HOSTNAME=$(minikube ip).nip.io
    TOKEN_ENDPOINT="https://${KEYCLOAK_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
    export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
  fi
}
