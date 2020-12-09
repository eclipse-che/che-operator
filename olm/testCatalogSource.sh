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

# bash ansi colors

GREEN='\033[0;32m'
NC='\033[0m'

readlink -f "$0"

if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "$0")

  OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")");
fi
echo "Operator repo path is ${OPERATOR_REPO}"

OLM_DIR="${OPERATOR_REPO}/olm"
export OPERATOR_REPO

# Function which will print all arguments need it to run this script
printHelp() {
  echo ''
  echo 'Please consider to pass this values to the script to run catalog source tests in your cluster:'
	echo '    PLATFORM                 - Platform used to run olm files tests'
	echo '    CHANNEL                  - Channel used to tests olm files'
	echo '    NAMESPACE                - Namespace where Eclipse Che will be deployed'
	echo '    INSTALLATION_TYPE        - Olm tests now includes two types of installation: Catalog source and marketplace'
	echo '    CATALOG_SOURCE_IMAGE     - Image name used to create a catalog source in cluster'
  echo ''
  echo 'EXAMPLE of running: ${OPERATOR_REPO}/olm/testCatalogSource.sh openshift nightly che catalog my_image_name'
}

# Check if a platform was defined...
PLATFORM=$1
if [ "${PLATFORM}" == "" ]; then
  echo -e "${RED}[ERROR]: Please specify a valid platform. The posible platforms are kubernetes or openshift.The script will exit with code 1.${NC}"
  printHelp
  exit 1
else
  echo "[INFO]: Successfully validated platform. Starting olm tests in platform: ${PLATFORM}."
fi

# Check if a channel was defined... The available channels are nightly and stable
CHANNEL=$2
if [ "${CHANNEL}" == "stable" ] || [ "${CHANNEL}" == "nightly" ]; then
  echo "[INFO]: Successfully validated operator channel. Starting olm tests in channel: ${CHANNEL}."
else
  echo "[ERROR]: Please specify a valid channel. The posible channels are stable and nightly.The script will exit with code 1."
  printHelp
  exit 1
fi

# Check if a namespace was defined...
NAMESPACE=$3
if [ "${NAMESPACE}" == "" ]; then
  echo "[ERROR]: No namespace was specified... The script wil exit with code 1."
  printHelp
  exit 1
else
  echo "[INFO]: Successfully asigned namespace ${NAMESPACE} to tests olm files."
fi

# Check if a INSTALLATION_TYPE was defined... The possible installation are marketplace or catalog source
INSTALLATION_TYPE=$4
if [ "${INSTALLATION_TYPE}" == "" ]; then
  echo "[ERROR]: Please specify a valid installation type. The valid values are: 'catalog' or 'marketplace'"
  printHelp
  exit 1
else
  echo "[INFO]: Successfully detected installation type: ${INSTALLATION_TYPE}"
fi

# Assign catalog source image
CATALOG_SOURCE_IMAGE=$5

IMAGE_REGISTRY_USER_NAME=${IMAGE_REGISTRY_USER_NAME:-eclipse}
echo "[INFO] Image 'IMAGE_REGISTRY_USER_NAME': ${IMAGE_REGISTRY_USER_NAME}"

init() {
  if [[ "${PLATFORM}" == "openshift" ]]
  then
    export PLATFORM=openshift
    PACKAGE_NAME=eclipse-che-preview-openshift
    PACKAGE_FOLDER_PATH="${OLM_DIR}/eclipse-che-preview-openshift/deploy/olm-catalog/${PACKAGE_NAME}"
  else
    PACKAGE_NAME=eclipse-che-preview-${PLATFORM}
    PACKAGE_FOLDER_PATH="${OLM_DIR}/eclipse-che-preview-${PLATFORM}/deploy/olm-catalog/${PACKAGE_NAME}"
  fi

  if [ "${CHANNEL}" == "nightly" ]; then
    PACKAGE_FOLDER_PATH="${OPERATOR_REPO}/deploy/olm-catalog/eclipse-che-preview-${PLATFORM}"
    CLUSTER_SERVICE_VERSION_FILE="${OPERATOR_REPO}/deploy/olm-catalog/eclipse-che-preview-${PLATFORM}/manifests/che-operator.clusterserviceversion.yaml"
    PACKAGE_VERSION=$(yq -r ".spec.version" "${CLUSTER_SERVICE_VERSION_FILE}")
  else
    PACKAGE_FILE_PATH="${PACKAGE_FOLDER_PATH}/${PACKAGE_NAME}.package.yaml"
    CLUSTER_SERVICE_VERSION=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${PACKAGE_FILE_PATH}")
    PACKAGE_VERSION=$(echo "${CLUSTER_SERVICE_VERSION}" | sed -e "s/${PACKAGE_NAME}.v//")
  fi

  source "${OLM_DIR}/olm.sh" "${PLATFORM}" "${PACKAGE_VERSION}" "${NAMESPACE}" "${INSTALLATION_TYPE}"

  if [ "${CHANNEL}" == "nightly" ]; then
    installOPM
  fi
}

buildOLMImages() {
  # Manage catalog source for every platform in part.
  # 1. Kubernetes:
  #    a) Use Minikube cluster. Enable registry addon, build catalog source and olm bundle images, push them to embedded private registry.
  #    b) Provide image registry env variables to push images to the real public registry(docker.io, quay.io etc).
  if [[ "${PLATFORM}" == "kubernetes" ]]
  then
    echo "[INFO]: Kubernetes platform detected"

    # Build operator image
    if [ -n "${OPERATOR_IMAGE}" ];then
      echo "[INFO]: Build operator image ${OPERATOR_IMAGE}..."
      cd "${OPERATOR_REPO}" && docker build -t "${OPERATOR_IMAGE}" -f Dockerfile .

      # Use operator image in the latest CSV
      if [ "${CHANNEL}" == "nightly" ]; then
        sed -i 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|' "${CLUSTER_SERVICE_VERSION_FILE}"
      else
        sed -i 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|' "${PACKAGE_FOLDER_PATH}/${PACKAGE_VERSION}/${PACKAGE_NAME}.v${PACKAGE_VERSION}.clusterserviceversion.yaml"
      fi
    fi

    CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che_operator_bundle:0.0.1"
    CATALOG_SOURCE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/testing_catalog:0.0.1"

    if [ "${CHANNEL}" == "nightly" ]; then
      echo "[INFO] Build bundle image... ${CATALOG_BUNDLE_IMAGE}"
      buildBundleImage "${CATALOG_BUNDLE_IMAGE}"
      echo "[INFO] Build catalog image... ${CATALOG_BUNDLE_IMAGE}"
      buildCatalogImage "${CATALOG_SOURCE_IMAGE}" "${CATALOG_BUNDLE_IMAGE}"
    fi

    echo "[INFO]: Successfully created catalog source container image and enabled minikube ingress."
  else
    echo "[ERROR]: Error to start olm tests. Invalid Platform"
    printHelp
    exit 1
  fi
}

run() {
  createNamespace
  if [ ! ${PLATFORM} == "openshift" ] && [ "${CHANNEL}" == "nightly" ]; then
    forcePullingOlmImages "${CATALOG_BUNDLE_IMAGE}"
  fi

  installOperatorMarketPlace
  subscribeToInstallation

  installPackage
  applyCRCheCluster
  waitCheServerDeploy
}

init
buildOLMImages
run
echo -e "\u001b[32m Done. \u001b[0m"
