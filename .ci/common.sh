#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

# Create CheCluster object in Openshift ci with desired values
function applyCRCheCluster() {
  echo "Creating Custom Resource"
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${CSV_FILE}")
  CR=$(echo "$CRs" | yq -r ".[0]")
  if [ "${PLATFORM}" == "openshift" ] && [ "${OAUTH}" == "false" ]; then
    CR=$(echo "$CR" | yq -r ".spec.auth.openShiftoAuth = false")
  fi
  if [ "${CHE_EXPOSURE_STRATEGY}" == "single-host" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.server.serverExposureStrategy = \"${CHE_EXPOSURE_STRATEGY}\"")
  fi
  echo -e "$CR"
  echo "$CR" | oc apply -n "${NAMESPACE}" -f -
}

# Wait for CheCluster object to be ready
function waitCheServerDeploy() {
  echo "[INFO] Waiting for Che server to be deployed"
  set +e

  i=0
  while [[ $i -le 480 ]]
  do
    status=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o jsonpath={.status.cheClusterRunning})
    echo -e ""
    echo -e "[INFO] Che deployment status:"
    oc get pods -n "${NAMESPACE}"
    if [ "${status:-UNAVAILABLE}" == "Available" ]
    then
      break
    fi
    sleep 10
    ((i++))
  done

  if [ $i -gt 480 ]
  then
    echo "[ERROR] Che server did't start after 8 minutes"
    exit 1
  fi
}

# Utility to wait for a workspace to be started after workspace:create.
function waitWorkspaceStart() {
  set +e
  chectl auth:login --chenamespace=${NAMESPACE} -u admin -p admin 

  export x=0
  while [ $x -le 180 ]
  do
    chectl workspace:list --chenamespace=${NAMESPACE}
    workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})
    workspaceStatus=$(echo "$workspaceList" | grep RUNNING | awk '{ print $4} ')
    echo -e ""

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
    echo -e "[ERROR] Workspace didn't start after 3 minutes."
    exit 1
  fi
}

function startNewWorkspace() {
  # Create and start a workspace
  sleep 5s
  chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
  chectl workspace:create --chenamespace=${NAMESPACE} --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml
}
