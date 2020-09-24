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

  # Create tmp folder to save "operator" installer templates
  mkdir -p "${OPERATOR_REPO}/tmp" && chmod 777 "${OPERATOR_REPO}/tmp"
  cp -rf "${OPERATOR_REPO}/deploy" "${OPERATOR_REPO}/tmp/che-operator"
}

function installLatestCheStable() {
  # Get Stable and new release versions from olm files openshift.
  export packageName=eclipse-che-preview-${PLATFORM}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${packageFilePath}")
  export lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")
  export previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
  export previousPackageVersion=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

  # Add stable Che images and tag to CR
  sed -i'.bak' -e "s/cheImage: ''/cheImage: quay.io\/eclipse\/che-server/" "${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml"
  sed -i'.bak' -e "s/cheImageTag: ''/cheImageTag: ${previousPackageVersion}/" "${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml"
  
  # set 'openShiftoAuth: false'
  sed -i'.bak' -e "s/openShiftoAuth: .*/openShiftoAuth: false/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml
  cat ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml

  # Change operator images defaults in the deployment
  sed -i'.bak' -e "s|nightly|${previousPackageVersion}|" "${OPERATOR_REPO}/tmp/che-operator/operator.yaml"
  cat "${OPERATOR_REPO}/tmp/che-operator/operator.yaml"

  # Start last stable version of che
  chectl server:start --platform=minishift --skip-kubernetes-health-check \
    --che-operator-cr-yaml="${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml" --templates="${OPERATOR_REPO}/tmp" \
    --installer=operator
}

# Utility to wait for new release to be up
function waitForNewCheVersion() {
  export n=0

  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheVersion}")
    cheIsRunning=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheClusterRunning}" )  
    oc get pods -n ${NAMESPACE}
    if [ "${cheVersion}" == "${lastPackageVersion}" ] && [ "${cheIsRunning}" == "Available" ]
    then
      echo -e "\u001b[32m Installed latest version che-operator: ${lastCSV} \u001b[0m"
      break
    fi
    sleep 6
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "Latest version install for Eclipse che failed."
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
  installLatestCheStable

  # Create an workspace
  getCheAcessToken # Function from ./util/ci_common.sh
  chectl workspace:create --devfile=${OPERATOR_REPO}/.ci/util/devfile-test.yaml

  # Change operator images defaults in the deployment
  sed -i'.bak' -e "s|${previousPackageVersion}|${lastPackageVersion}|" "${OPERATOR_REPO}/tmp/che-operator/operator.yaml"

  # Update the operator to the new release
  chectl server:update --skip-version-check --installer=operator --platform=minishift --templates="${OPERATOR_REPO}/tmp"

  oc patch checluster eclipse-che --type='json' -p='[{"op": "replace", "path": "/spec/server/cheImageTag", "value":"'${lastPackageVersion}'"}]' -n ${NAMESPACE}
  waitForNewCheVersion

  getCheAcessToken # Function from ./util/ci_common.sh
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
