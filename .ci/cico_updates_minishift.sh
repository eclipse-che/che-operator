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

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Catch_Finish is executed after finish script.
catchFinish() {
  result=$?

  if [ "$result" != "0" ]; then
    echo "[ERROR] Please check the artifacts in github actions"
    getOCCheClusterLogs
    exit 1
  fi

  echo "[INFO] Job finished Successfully.Please check the artifacts in github actions"
  getOCCheClusterLogs

  exit $result
}

function init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export PLATFORM="openshift"
  export NAMESPACE="che"
  export CHANNEL="stable"

  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

  # Get Stable and new release versions from olm files openshift.
  export packageName=eclipse-che-preview-${PLATFORM}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${packageFilePath}")
  export lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")
  export previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
  export previousPackageVersion=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

  export lastOperatorPath=${OPERATOR_REPO}/tmp/${lastPackageVersion}
  export previousOperatorPath=${OPERATOR_REPO}/tmp/${previousPackageVersion}

  export lastOperatorTemplate=${lastOperatorPath}/chectl/templates
  export previousOperatorTemplate=${previousOperatorPath}/chectl/templates

  rm -rf tmp
  # Create tmp folder to save "operator" installer templates
  mkdir -p "${OPERATOR_REPO}/tmp" && chmod 777 "${OPERATOR_REPO}/tmp"

  # clone the exact versions to use their templates
  git clone --depth 1 --branch ${previousPackageVersion} https://github.com/eclipse/che-operator/ ${previousOperatorPath}
  git clone --depth 1 --branch ${lastPackageVersion} https://github.com/eclipse/che-operator/ ${lastOperatorPath}

  # chectl requires 'che-operator' template folder
  mkdir -p "${lastOperatorTemplate}/che-operator"
  mkdir -p "${previousOperatorTemplate}/che-operator"

  cp -rf ${previousOperatorPath}/deploy/* "${previousOperatorTemplate}/che-operator"
  cp -rf ${lastOperatorPath}/deploy/* "${lastOperatorTemplate}/che-operator"

  # set 'openShiftoAuth: false'
  sed -i'.bak' -e "s/openShiftoAuth: .*/openShiftoAuth: false/" "${previousOperatorTemplate}/che-operator/crds/org_v1_che_cr.yaml"
  sed -i'.bak' -e "s/openShiftoAuth: .*/openShiftoAuth: false/" "${lastOperatorTemplate}/che-operator/crds/org_v1_che_cr.yaml"
}

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

# Utility to get che events and pod logs from openshift
function getOCCheClusterLogs() {
  mkdir -p /tmp/artifacts-che
  cd /tmp/artifacts-che

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

function minishiftUpdates() {
  # Install previous stable version of Eclipse Che
  installPreviousStableChe

  # Create an workspace
  getCheAcessToken # Function from ./util/ci_common.sh
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
  oc get events -n ${NAMESPACE}
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
minishiftUpdates
