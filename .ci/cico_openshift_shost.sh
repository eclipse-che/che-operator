#!/bin/bash
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

set -ex

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Catch_Finish is executed after finish script.
catchFinish() {
  result=$?

  if [ "$result" != "0" ]; then
    echo "[ERROR] Please check the openshift ci artifacts"
    getOCCheClusterLogs
    exit 1
  fi

  echo "[INFO] Job finished Successfully.Please check openshift ci artifacts"
  getOCCheClusterLogs

  exit $result
}

# Define global environments
function init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export NAMESPACE="che"
  export PLATFORM="openshift"

  # Set operator root directory
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    export OPERATOR_REPO=${WORKSPACE};
  else
    export OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

    # CHE_OPERATOR_IMAGE is exposed in openshift ci pod. This image is build in every job and used then to deploy Che
    # More info about how images are builded in Openshift CI: https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
    export OPERATOR_IMAGE=${CHE_OPERATOR_IMAGE}
}

# Get Token from single host mode deployment
function getSingleHostToken() {
    export KEYCLOAK_HOSTNAME=$(oc get routes/che -n ${NAMESPACE} -o jsonpath='{.spec.host}')
    export TOKEN_ENDPOINT="https://${KEYCLOAK_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
    export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
}

# Utility to wait for a workspace to be started after workspace:create.
function waitSingleHostWorkspaceStart() {
  set +e
  export x=0
  while [ $x -le 180 ]
  do
    getSingleHostToken

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

# Utility to get che events and pod logs from openshift cluster
function getOCCheClusterLogs() {
  mkdir -p /tmp/artifacts/artifacts-che
  cd /tmp/artifacts/artifacts-che

  for POD in $(oc get pods -o name -n ${NAMESPACE}); do
    for CONTAINER in $(oc get -n ${NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
      echo ""
      echo "[INFO] Getting logs from $POD"
      echo ""
      oc logs ${POD} -c ${CONTAINER} -n ${NAMESPACE} |tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
    done
  done
  echo "[INFO] Get events"
  oc get events -n ${NAMESPACE}| tee get_events.log
  oc get all | tee get_all.log
}

# Deploy Eclipse Che in single host mode
function run() {
    # Patch file to pass to chectl
    cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  server:
    serverExposureStrategy: 'single-host'
    singleHostExposureType: 'gateway'
  auth:
    updateAdminPassword: false
    openShiftoAuth: false
EOL
    echo "======= Che cr patch ======="
    cat /tmp/che-cr-patch.yaml

    # Start to deploy Che
    chectl server:start --platform=openshift --skip-kubernetes-health-check --installer=operator --chenamespace=${NAMESPACE} --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml --che-operator-image=${OPERATOR_IMAGE}

    # Create and start a workspace
    getSingleHostToken # Function from ./util/ci_common.sh
    chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    # Wait for workspace to be up
    waitSingleHostWorkspaceStart
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
run
