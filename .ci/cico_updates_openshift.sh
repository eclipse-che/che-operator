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

init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")

  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    export OPERATOR_REPO=${WORKSPACE};
  else
    export OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

  export PLATFORM="openshift"
  export NAMESPACE="che"
  export CHANNEL="stable"
}

# Utility to wait for Eclipse Che to be up in Openshift
function waitCheUpdateInstall() {
  export packageName=eclipse-che-preview-${PLATFORM}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${packageFilePath}")
  export lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")

  echo -e "\u001b[34m Check installation last version che-operator...$lastPackageVersion \u001b[0m"

  export n=0

  while [ $n -le 360 ]
  do
    cheVersion=$(kubectl get checluster/eclipse-che -n "${NAMESPACE}" -o jsonpath={.status.cheVersion})
    if [ "${cheVersion}" == $lastPackageVersion ]
    then
      echo -e "\u001b[32m Installed latest version che-operator: ${lastCSV} \u001b[0m"
      break
    fi
    sleep 3
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "[ERROR] Latest version install for Eclipse Che failed."
    exit 1
  fi
}

function openshiftUpdates() {
  "${OPERATOR_REPO}"/olm/testUpdate.sh ${PLATFORM} ${CHANNEL} ${NAMESPACE}

  getCheAcessToken
  chectl workspace:create --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

  waitCheUpdateInstall
  getCheAcessToken

  local cheVersion=$(kubectl get checluster/eclipse-che -n "${NAMESPACE}" -o jsonpath={.status.cheVersion})

  echo "[INFO] Successfully installed Eclipse Che: ${cheVersion}"
  sleep 120

  getCheAcessToken
  workspaceList=$(chectl workspace:list)
  workspaceID=$(echo "$workspaceList" | grep -oP '\bworkspace.*?\b')
  chectl workspace:start $workspaceID
  chectl workspace:list

  waitWorkspaceStart
  echo "[INFO] Successfully started an workspace on Eclipse Che: ${cheVersion}"
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
openshiftUpdates
