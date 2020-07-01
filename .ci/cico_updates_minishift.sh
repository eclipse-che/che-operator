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
  SCRIPT=$(readlink -f "$0")
  SCRIPT_DIR=$(dirname "$SCRIPT")

  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

  RAM_MEMORY=8192
  PLATFORM="openshift"
  NAMESPACE="che"
}

installDependencies() {
  installYQ
  installJQ
  install_VirtPackages
  installStartDocker
  start_libvirt
  minishift_installation
  installChectl
  load_jenkins_vars
}

installCheUpdates() {
  export packageName=eclipse-che-preview-${PLATFORM}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${packageFilePath}")
  export lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")
  export previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
  export previousPackageVersion=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

  chectl server:start --platform=minishift --che-operator-cr-patch-yaml=${OPERATOR_REPO}/.ci/util/cr-test.yaml --installer=operator \
    --cheimage=quay.io/eclipse/che-server:${previousPackageVersion} --che-operator-image=quay.io/eclipse/che-operator:${previousPackageVersion}
}

testUpdates() {
  # Install previous stable version of Eclipse Che
  installCheUpdates

  # Create an workspace
  getCheAcessToken # Function from ./util/ci_common.sh
  chectl workspace:create --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

  # Create tmp folder and add che operator templates used by server:update command.
  mkdir -p "$OPERATOR_REPO/tmp" && chmod 777 "$OPERATOR_REPO/tmp"
  cp -r deploy "$OPERATOR_REPO/tmp/che-operator"

  # Update the server to the latest stable che version
  chectl server:update --skip-version-check --installer=operator --platform=minishift \
    --che-operator-image=quay.io/eclipse/che-operator:${lastPackageVersion} --templates="$OPERATOR_REPO/tmp"
  echo "[INFO] Successfull installed update in minishift"

  # Start an workspace
  getCheAcessToken
  workspaceList=$(chectl workspace:list)
  workspaceID=$(echo "$workspaceList" | grep -oP '\bworkspace.*?\b')
  chectl workspace:start $workspaceID

  # Wait for workspace to be up
  waitWorkspaceStart  # Function from ./util/ci_common.sh
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
#installDependencies
testUpdates
