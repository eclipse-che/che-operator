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
  export NAMESPACE="eclipse-che"
  export ARTIFACTS_DIR="/tmp/artifacts-che"
  export TEMPLATES=${OPERATOR_REPO}/tmp
  export OPERATOR_IMAGE="quay.io/eclipse/che-operator:test"

  export CHE_EXPOSURE_STRATEGY="multi-host"
  export OAUTH="false"

  export XDG_DATA_HOME=/tmp/chectl/data
  export XDG_CACHE_HOME=/tmp/chectl/cache
  export XDG_CONFIG_HOME=/tmp/chectl/config

  export OPENSHIFT_NIGHTLY_CSV_FILE="${OPERATOR_REPO}/deploy/olm-catalog/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"

  # prepare templates directory
  rm -rf ${TEMPLATES}
  mkdir -p "${TEMPLATES}/che-operator" && chmod 777 "${TEMPLATES}"
}

initLatestTemplates() {
  cp -rf ${OPERATOR_REPO}/deploy/* "${TEMPLATES}/che-operator"
}

initStableTemplates() {
  local platform=$1
  local channel=$2

  # Get Stable and new release versions from olm files openshift.
  export packageName=eclipse-che-preview-${platform}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
  export LAST_PACKAGE_VERSION=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")

  export previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${LAST_PACKAGE_VERSION}/${packageName}.v${LAST_PACKAGE_VERSION}.clusterserviceversion.yaml")
  export PREVIOUS_PACKAGE_VERSION=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

  export lastOperatorPath=${OPERATOR_REPO}/tmp/${LAST_PACKAGE_VERSION}
  export previousOperatorPath=${OPERATOR_REPO}/tmp/${PREVIOUS_PACKAGE_VERSION}

  export LAST_OPERATOR_TEMPLATE=${lastOperatorPath}/chectl/templates
  export PREVIOUS_OPERATOR_TEMPLATE=${previousOperatorPath}/chectl/templates

  # clone the exact versions to use their templates
  git clone --depth 1 --branch ${PREVIOUS_PACKAGE_VERSION} https://github.com/eclipse/che-operator/ ${previousOperatorPath}
  git clone --depth 1 --branch ${LAST_PACKAGE_VERSION} https://github.com/eclipse/che-operator/ ${lastOperatorPath}

  # chectl requires 'che-operator' template folder
  mkdir -p "${LAST_OPERATOR_TEMPLATE}/che-operator"
  mkdir -p "${PREVIOUS_OPERATOR_TEMPLATE}/che-operator"

  cp -rf ${previousOperatorPath}/deploy/* "${PREVIOUS_OPERATOR_TEMPLATE}/che-operator"
  cp -rf ${lastOperatorPath}/deploy/* "${LAST_OPERATOR_TEMPLATE}/che-operator"
}

# Utility to wait for a workspace to be started after workspace:create.
waitWorkspaceStart() {
  set +e
  export x=0
  while [ $x -le 180 ]
  do
    chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
    chectl workspace:list --chenamespace=${NAMESPACE}
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
  chectl server:logs --chenamespace=${NAMESPACE} --directory=${ARTIFACTS_DIR}
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
    --skip-kubernetes-health-check \
    --che-operator-cr-yaml ${templates}/che-operator/crds/org_v1_che_cr.yaml \
    --templates ${templates}
}

waitEclipseCheDeployed() {
  local version=$1
  export n=0

  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheVersion}")
    cheIsRunning=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheClusterRunning}" )
    oc get pods -n ${NAMESPACE}
    if [ "${cheVersion}" == "${version}" ] && [ "${cheIsRunning}" == "Available" ]
    then
      echo -e "\u001b[32m Eclipse Che ${version} has been succesfully deployed \u001b[0m"
      break
    fi
    sleep 6
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "Failed to deploy Eclipse Che ${version}"
    exit 1
  fi
}

updateEclipseChe() {
  local image=$1
  local templates=$2

  chectl server:update --chenamespace=${NAMESPACE} -y --che-operator-image=${image} --templates=${templates}
}

startNewWorkspace() {
  # Create and start a workspace
  sleep 5s
  chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
  chectl workspace:create --start --chenamespace=${NAMESPACE} --devfile=$OPERATOR_REPO/.ci/devfile-test.yaml
}

createWorkspace() {
  sleep 5s
  chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
  chectl workspace:create --chenamespace=${NAMESPACE} --devfile=${OPERATOR_REPO}/.ci/devfile-test.yaml
}

startExistedWorkspace() {
  sleep 5s
  chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
  chectl workspace:list --chenamespace=${NAMESPACE}
  workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})

  # Grep applied to MacOS
  workspaceID=$(echo "$workspaceList" | grep workspace | awk '{ print $1} ')
  workspaceID="${workspaceID%'ID'}"
  echo "[INFO] Workspace id of created workspace is: ${workspaceID}"

  chectl workspace:start $workspaceID
}

disableOpenShiftOAuth() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.auth.openShiftoAuth = false' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

disableUpdateAdminPassword() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.auth.updateAdminPassword = false' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setServerExposureStrategy() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.server.serverExposureStrategy = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setSingleHostExposureType() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.k8s.singleHostExposureType = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setIngressDomain() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.k8s.ingressDomain = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setCustomOperatorImage() {
  local file="${1}/che-operator/operator.yaml"
  yq -rSY '.spec.template.spec.containers[0].image = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
  yq -rSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

insecurePrivateDockerRegistry() {
  IMAGE_REGISTRY_HOST="0.0.0.0:5000"
  export IMAGE_REGISTRY_HOST

  local dockerDaemonConfig="/etc/docker/daemon.json"
  sudo mkdir -p "/etc/docker"
  sudo touch "${dockerDaemonConfig}"

  config="{\"insecure-registries\" : [\"${IMAGE_REGISTRY_HOST}\"]}"
  echo "${config}" | sudo tee "${dockerDaemonConfig}"

  if [ -x "$(command -v docker)" ]; then
      echo "[INFO] Restart docker daemon to set up private registry info."
      sudo service docker restart
  fi
}

# Utility to print objects created by Openshift CI automatically
printOlmCheObjects() {
  echo -e "[INFO] Operator Group object created in namespace: ${NAMESPACE}"
  oc get operatorgroup -n "${NAMESPACE}" -o yaml

  echo -e "[INFO] Catalog Source object created in namespace: ${NAMESPACE}"
  oc get catalogsource -n "${NAMESPACE}" -o yaml

  echo -e "[INFO] Subscription object created in namespace: ${NAMESPACE}"
  oc get subscription -n "${NAMESPACE}" -o yaml
}

# Patch subscription with image builded from source in Openshift CI job.
patchEclipseCheOperatorSubscription() {
  OPERATOR_POD=$(oc get pods -o json -n ${NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name')
  oc patch pod ${OPERATOR_POD} -n ${NAMESPACE} --type='json' -p='[{"op": "replace", "path": "/spec/containers/0/image", "value":'${OPERATOR_IMAGE}'}]'

  # The following command retrieve the operator image
  OPERATOR_POD_IMAGE=$(oc get pods -n ${NAMESPACE} -o json | jq -r '.items[] | select(.metadata.name | test("che-operator-")).spec.containers[].image')
  echo -e "[INFO] CHE operator image is ${OPERATOR_POD_IMAGE}"
}

# Create CheCluster object in Openshift ci with desired values
applyOlmCR() {
  echo "Creating Custom Resource"

  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${OPENSHIFT_NIGHTLY_CSV_FILE}")
  CR=$(echo "$CRs" | yq -r ".[0]")
  CR=$(echo "$CR" | yq -r ".spec.auth.openShiftoAuth = ${OAUTH}")
  CR=$(echo "$CR" | yq -r ".spec.server.serverExposureStrategy = \"${CHE_EXPOSURE_STRATEGY}\"")

  echo -e "$CR"
  echo "$CR" | oc apply -n "${NAMESPACE}" -f -
}
