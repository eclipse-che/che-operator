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

# Detect the base directory where che-operator is cloned
SCRIPT=$(readlink -f "$0")
export SCRIPT

ROOT_DIR=$(dirname "$(dirname "$SCRIPT")");
OLM_DIR="${ROOT_DIR}/olm"
export ROOT_DIR

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
  echo 'EXAMPLE of running: ${ROOT_DIR}/olm/testCatalogSource.sh crc nightly che catalog my_image_name'
  echo ''
  echo -e "${GREEN}!!!ATTENTION!!! To run in your local machine the script, please change PLATFORM VARIABLE to crc"
  echo -e "${GREEN} olm test in CRC cluster.${NC}"
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

init() {
  # GET the package version to apply. In case of CRC we should detect somehow the platform is openshift to get packageversion
  if [[ "${PLATFORM}" == "crc" ]]
  then
    PACKAGE_NAME=eclipse-che-preview-openshift
    PACKAGE_FOLDER_PATH="${OLM_DIR}/eclipse-che-preview-openshift/deploy/olm-catalog/${PACKAGE_NAME}"
  else
    PACKAGE_NAME=eclipse-che-preview-${PLATFORM}
    PACKAGE_FOLDER_PATH="${OLM_DIR}/eclipse-che-preview-${PLATFORM}/deploy/olm-catalog/${PACKAGE_NAME}"
  fi

  if [ "${CHANNEL}" == "nightly" ]; then
    CLUSTER_SERVICE_VERSION="${ROOT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${PLATFORM}/manifests/che-operator.clusterserviceversion.yaml"
    PACKAGE_VERSION=$(yq -r ".spec.version" "${CLUSTER_SERVICE_VERSION}")
  else
    PACKAGE_FILE_PATH="${PACKAGE_FOLDER_PATH}/${PACKAGE_NAME}.package.yaml"
    CLUSTER_SERVICE_VERSION=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${PACKAGE_FILE_PATH}")
    PACKAGE_VERSION=$(echo "${CLUSTER_SERVICE_VERSION}" | sed -e "s/${PACKAGE_NAME}.v//")
  fi
 
  # Manage catalog source for every platform in part.
  # 1.Kubernetes: We need to eval minikube docker image and build there the catalog source
  # 2.Openshift: Openshift platform will be run as part of Openshift CI and the catalog source will be build automatically and exposed
  # 3.CRC: To run in our Code Ready Container Cluster we need have installed podman and running crc cluster...
  if [[ "${PLATFORM}" == "kubernetes" ]]
  then
    echo "[INFO]: Kubernetes platform detected. Starting to build catalog source image..."
    eval "$(minikube -p minikube docker-env)"

    # docker build -t ${CATALOG_SOURCE_IMAGE} -f "${ROOT_DIR}"/eclipse-che-preview-"${PLATFORM}"/Dockerfile \
    # "${ROOT_DIR}"/eclipse-che-preview-"${PLATFORM}"

    minikube addons enable ingress
    echo "[INFO]: Successfully created catalog cource container image and enabled minikube ingress."

  elif [[ "${PLATFORM}" == "openshift" ]]
  then
    echo "[INFO]: Catalog Source container image to run olm tests in openshift platform is: ${CATALOG_SOURCE_IMAGE}"

  elif [[ "${PLATFORM}" == "crc" ]]
  then
    echo "[INFO]: Starting to build catalog image and push to CRC ImageStream."
    export PLATFORM="openshift"

    oc login -u kubeadmin -p $(crc console --credentials | awk -F "kubeadmin" '{print $2}' | cut -c 5- | rev | cut -c31- | rev) https://api.crc.testing:6443
    oc new-project ${NAMESPACE}

    # Get Openshift Image registry host
    IMAGE_REGISTRY_HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
    podman login -u kubeadmin -p $(oc whoami -t) ${IMAGE_REGISTRY_HOST} --tls-verify=false

    podman build -t ${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_SOURCE_IMAGE} -f "${ROOT_DIR}"/eclipse-che-preview-"${PLATFORM}"/Dockerfile \
        "${ROOT_DIR}"/eclipse-che-preview-"${PLATFORM}"
    podman push ${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_SOURCE_IMAGE}:latest --tls-verify=false

    # For some reason CRC external registry exposed is not working. I'll use the internal registry in cluster which is:image-registry.openshift-image-registry.svc:5000
    export CATALOG_SOURCE_IMAGE=image-registry.openshift-image-registry.svc:5000/${NAMESPACE}/${CATALOG_SOURCE_IMAGE}
    echo "[INFO]: Successfully added catalog source image to crc image registry: ${CATALOG_SOURCE_IMAGE}"

  else
    echo "[ERROR]: Error to start olm tests. Invalid Platform"
    printHelp
    exit 1
  fi
}

run() {
  if [ -z "${IMAGE_REGISTRY}" ]; then
    IMAGE_REGISTRY="quay.io"
    echo "Image registry: ${IMAGE_REGISTRY}"
  fi
  source "${OLM_DIR}/olm.sh" "${PLATFORM}" "${PACKAGE_VERSION}" "${NAMESPACE}" "${INSTALLATION_TYPE}"

  installOPM
  # loginToImageRegistry

  OPM_BUNDLE_DIR="${ROOT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
  OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${IMAGE_REGISTRY}/che_operator_bundle:0.0.1"
  echo "Try to build images..."
  buildBundleImage "${OPM_BUNDLE_MANIFESTS_DIR}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  CATALOG_IMAGENAME="${IMAGE_REGISTRY}/testing_catalog:0.0.1"
  buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  createNamespace
  forcePullingOlmImages "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"
  installOperatorMarketPlace
  subscribeToInstallation

  installPackage
  applyCRCheCluster
  waitCheServerDeploy
}

init
run
echo "[INFO] Done."
