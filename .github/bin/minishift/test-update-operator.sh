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

set -e
set -x
set -u

export OPERATOR_REPO=$(dirname $(dirname $(dirname $(dirname $(readlink -f "$0")))))
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

function installPreviousStableChe() {
  cat "${previousOperatorTemplate}/che-operator/crds/org_v1_che_cr.yaml"

  # Start last stable version of che
  chectl server:deploy --platform=minishift  \
    --che-operator-image=quay.io/eclipse/che-operator:${previousPackageVersion} \
    --che-operator-cr-yaml="${previousOperatorTemplate}/che-operator/crds/org_v1_che_cr.yaml" \
    --templates="${previousOperatorTemplate}" \
    --installer=operator
}

function waitForNewCheVersion() {
  export n=0

  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheVersion}")
    cheIsRunning=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheClusterRunning}" )
    oc get pods -n ${NAMESPACE}
    if [ "${cheVersion}" == "${lastPackageVersion}" ] && [ "${cheIsRunning}" == "Available" ]
    then
      echo -e "\u001b[32m The latest Eclipse Che ${lastCSV} has been deployed \u001b[0m"
      break
    fi
    sleep 6
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "Failed to deploy the latest ${lastCSV} Eclipse Che."
    exit 1
  fi
}

prepareTemplates() {
  # set 'openShiftoAuth: false'
  sed -i'.bak' -e "s/openShiftoAuth: .*/openShiftoAuth: false/" "${previousOperatorTemplate}/che-operator/crds/org_v1_che_cr.yaml"
  sed -i'.bak' -e "s/openShiftoAuth: .*/openShiftoAuth: false/" "${lastOperatorTemplate}/che-operator/crds/org_v1_che_cr.yaml"
}

runTest() {
  prepareTemplates

  # deployEclipseChe "operator" "minishift" ${OPERATOR_IMAGE} ${TEMPLATES}

  # Create an workspace
  chectl auth:login -u admin -p admin
  chectl workspace:create --devfile=${OPERATOR_REPO}/.ci/util/devfile-test.yaml

  # Update the operator to the new release
  chectl server:update -y \
    --che-operator-image=quay.io/eclipse/che-operator:${lastPackageVersion} \
    --templates="${lastOperatorTemplate}"

  waitForNewCheVersion

  # Sleep before starting a workspace
  sleep 10s

  chectl auth:login -u admin -p admin
  chectl workspace:list
  workspaceList=$(chectl workspace:list)

  # Grep applied to MacOS
  workspaceID=$(echo "$workspaceList" | grep workspace | awk '{ print $1} ')
  workspaceID="${workspaceID%'ID'}"
  echo "[INFO] Workspace id of created workspace is: ${workspaceID}"

  chectl workspace:start $workspaceID

  # Wait for workspace to be up
  waitWorkspaceStart  # Function from ./util/ci_common.sh
}

init
initStableTemplates
runTest
