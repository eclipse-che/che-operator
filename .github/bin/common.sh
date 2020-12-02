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

set -e
set -o pipefail
set -u
set -x

catchFinish() {
  result=$?

  collectCheLogWithChectl
  if [ "$result" != "0" ]; then
    echo "[ERROR] Job failed."
  else
    echo "[INFO] Job completed successfully."
  fi

  echo "[INFO] Please check github actions artifacts."
  exit $result
}

init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export NAMESPACE="che"
  export ARTIFACTS_DIR="/tmp/artifacts-che"
  export TEMPLATES=${OPERATOR_REPO}/tmp
  export OPERATOR_IMAGE="quay.io/eclipse/che-operator:test"

  # prepare templates directory
  rm -rf ${TEMPLATES}
  mkdir -p "${TEMPLATES}/che-operator" && chmod 777 "${TEMPLATES}"

  # install dependencies
  installYq
}

initLatestTemplates() {
  cp -rf ${OPERATOR_REPO}/deploy/* "${TEMPLATES}/che-operator"
}

initStableTemplates() {
    export PLATFORM="openshift"
  export NAMESPACE="che"
  export CHANNEL="stable"

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

  # clone the exact versions to use their templates
  git clone --depth 1 --branch ${previousPackageVersion} https://github.com/eclipse/che-operator/ ${previousOperatorPath}
  git clone --depth 1 --branch ${lastPackageVersion} https://github.com/eclipse/che-operator/ ${lastOperatorPath}

  # chectl requires 'che-operator' template folder
  mkdir -p "${lastOperatorTemplate}/che-operator"
  mkdir -p "${previousOperatorTemplate}/che-operator"

  cp -rf ${previousOperatorPath}/deploy/* "${previousOperatorTemplate}/che-operator"
  cp -rf ${lastOperatorPath}/deploy/* "${lastOperatorTemplate}/che-operator"
}

# Utility to wait for a workspace to be started after workspace:create.
waitWorkspaceStart() {
  set +e
  export x=0
  while [ $x -le 180 ]
  do
    getCheAcessToken

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

installYq() {
  YQ=$(command -v yq) || true
  if [[ ! -x "${YQ}" ]]; then
    pip3 install wheel
    pip3 install yq
  fi
  echo "[INFO] $(yq --version)"
  echo "[INFO] $(jq --version)"
}

# Graps Eclipse Che logs
collectCheLogWithChectl() {
  mkdir -p ${ARTIFACTS_DIR}
  chectl server:logs --directory=${ARTIFACTS_DIR}
}

# Build latest operator image
buildCheOperatorImage() {
  docker build -t "${OPERATOR_IMAGE}" -f Dockerfile . && docker save "${OPERATOR_IMAGE}" > operator.tar
}

copyCheOperatorImageToMinikube() {
  eval $(minikube docker-env) && docker load -i operator.tar && rm operator.tar
}

copyCheOperatorImageToMinishift() {
  eval $(minishift docker-env) && docker load -i operator.tar && rm operator.tar
}

deployEclipseChe() {
  local installer=$1
  local platform=$2
  local image=$3
  local templates=$4

  echo "[INFO] Eclipse Che custom resource"
  cat ${templates}/che-operator/crds/org_v1_che_cr.yaml

  echo "[INFO] Eclipse Che operator deployment"
  cat ${templates}/che-operator/operator.yaml

  chectl server:deploy \
    --platform=${platform} \
    --installer ${installer} \
    --chenamespace ${NAMESPACE} \
    --che-operator-image ${image} \
    --che-operator-cr-yaml ${templates}/che-operator/crds/org_v1_che_cr.yaml \
    --templates ${templates}
}

updateEclipseChe() {
  local image=$1
  local templates=$2

  chectl server:update -y --che-operator-image=${image} --templates=${templates}
}

startWorkspace() {
  # Create and start a workspace
  chectl auth:login -u admin -p admin
  chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

  # Wait for workspace to be up
  waitWorkspaceStart
}

disableOpenShiftOAuth() {
  yq -riSY  '.spec.auth.openShiftoAuth = false' "${1}/che-operator/crds/org_v1_che_cr.yaml"
}

disableUpdateAdminPassword() {
  yq -riSY  '.spec.auth.updateAdminPassword = false' "${1}/che-operator/crds/org_v1_che_cr.yaml"
}

setServerExposureStrategy() {
  yq -riSY  '.spec.server.serverExposureStrategy = '${2} "${1}/che-operator/crds/org_v1_che_cr.yaml"
}

setSingleHostExposureType() {
  yq -riSY  '.spec.k8s.singleHostExposureType = '${2} "${1}/che-operator/crds/org_v1_che_cr.yaml"
}

setCustomOperatorImage() {
  yq -riSY  '.spec.template.spec.containers[0].image = '${2} "${1}/che-operator/operator.yaml"
  yq -riSY  '.spec.template.spec.containers[0].imagePullPolicy = IfNotPresent' "${1}/che-operator/operator.yaml"
}
