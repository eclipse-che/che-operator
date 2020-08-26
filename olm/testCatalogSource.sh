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

echo "===================PATH to compare"
readlink -f "$0"

if [ -z "${OPERATOR_REPO}" ]; then
  # Detect the base directory where che-operator is cloned
  SCRIPT=$(readlink -f "$0")
  export SCRIPT # do we need to export it?

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
  echo 'EXAMPLE of running: ${OPERATOR_REPO}/olm/testCatalogSource.sh crc nightly che catalog my_image_name'
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

if [ -z "${IMAGE_REGISTRY_USER_NAME}" ]; then
  IMAGE_REGISTRY_USER_NAME=eclipse
fi
echo "[INFO] Image 'IMAGE_REGISTRY_USER_NAME': ${IMAGE_REGISTRY_USER_NAME}"

init() {
  # GET the package version to apply. In case of CRC we should detect somehow the platform is openshift to get packageversion
  if [[ "${PLATFORM}" == "crc" ]]
  then
    export IS_CRC="true"
    export PLATFORM=openshift
    PACKAGE_NAME=eclipse-che-preview-openshift
    PACKAGE_FOLDER_PATH="${OLM_DIR}/eclipse-che-preview-openshift/deploy/olm-catalog/${PACKAGE_NAME}"
  else
    PACKAGE_NAME=eclipse-che-preview-${PLATFORM}
    PACKAGE_FOLDER_PATH="${OLM_DIR}/eclipse-che-preview-${PLATFORM}/deploy/olm-catalog/${PACKAGE_NAME}"
  fi

  if [ "${CHANNEL}" == "nightly" ]; then
    CLUSTER_SERVICE_VERSION_FILE="${OPERATOR_REPO}/deploy/olm-catalog/che-operator/eclipse-che-preview-${PLATFORM}/manifests/che-operator.clusterserviceversion.yaml"
    PACKAGE_VERSION=$(yq -r ".spec.version" "${CLUSTER_SERVICE_VERSION_FILE}")
  else
    PACKAGE_FILE_PATH="${PACKAGE_FOLDER_PATH}/${PACKAGE_NAME}.package.yaml"
    CLUSTER_SERVICE_VERSION=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${PACKAGE_FILE_PATH}")
    PACKAGE_VERSION=$(echo "${CLUSTER_SERVICE_VERSION}" | sed -e "s/${PACKAGE_NAME}.v//")
  fi

  source "${OLM_DIR}/olm.sh" "${PLATFORM}" "${PACKAGE_VERSION}" "${NAMESPACE}" "${INSTALLATION_TYPE}"

  echo "${IS_CRC}"

  if [ "${CHANNEL}" == "nightly" ]; then
    installOPM
  fi
}

buildOLMImages() {
  # Manage catalog source for every platform in part.
  # 1. Kubernetes: We need to enable registry addon, build catalog images and push them to embedded private registry(Or we should provide image registry env to push images to real registry...).
  # 2. Openshift: Openshift platform will be run as part of Openshift CI and the catalog source will be build automatically and exposed
  # 3. CRC: To run in our Code Ready Container Cluster we need have installed podman and running crc cluster...
  if [[ "${PLATFORM}" == "kubernetes" ]]
  then
    echo "[INFO]: Kubernetes platform detected"
    eval "$(minikube docker-env)"

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

    loginToImageRegistry

    CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che_operator_bundle:0.0.1"
    CATALOG_SOURCE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/testing_catalog:0.0.1"

    echo "[INFO] Build bundle image... ${CATALOG_BUNDLE_IMAGE}"
    buildBundleImage "${CATALOG_BUNDLE_IMAGE}"
    echo "[INFO] Build catalog image... ${CATALOG_BUNDLE_IMAGE}"
    buildCatalogImage "${CATALOG_SOURCE_IMAGE}" "${CATALOG_BUNDLE_IMAGE}"

    minikube addons enable ingress
    echo "[INFO]: Successfully created catalog source container image and enabled minikube ingress."

  elif [[ "${IS_CRC}" == "false" ]]
  then
    echo "[INFO]: Catalog Source container image to run olm tests in openshift platform is: ${CATALOG_SOURCE_IMAGE}"

  elif [[ "${PLATFORM}" == "openshift" ]]
  then
    echo "[INFO]: Starting to build catalog image and push to CRC ImageStream."

    # ls /etc/boskos
    # if [ -n "$(cat '/etc/boskos/password')" ]; then
    #   echo "PSW file exists"
    # fi
    echo "============"
    oc whoami
    echo "============"
    # CRC_BINARY=$(command -v crc) || true
    if [[ "${OPENSHIFT_CI}" == "true" ]];then echo "Openshift ci!"; fi
    # if [[ ! "$(oc whoami  2>/dev/null)" =~ "kube:admin" ]] && [[ ! -x "${CRC_BINARY}" ]; then 
    #   oc login -u kubeadmin -p $(crc console --credentials | awk -F "kubeadmin" '{print $2}' | cut -c 5- | rev | cut -c31- | rev) https://api.crc.testing:6443
    # fi

    oc new-project "${NAMESPACE}" || true

    oc get route --all-namespaces
    echo "-------------------------------------------------"
    oc get configs.imageregistry.operator.openshift.io/cluster -o yaml
    echo "-------------------------------------------------"
    oc get route -n openshift-image-registry
    oc get pods -n openshift-image-registry

    echo "Registry pods:====="
    oc get pods -n openshift-image-registry

    if [ ! $(oc get configs.imageregistry.operator.openshift.io/cluster -o yaml | yq -r ".spec.defaultRoute") == true ];then
      oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
    fi

    echo "Registry pods:====="
    oc get pods -n openshift-image-registry

    exit 0

    sleep 15

    # Get Openshift Image registry host
    IMAGE_REGISTRY_HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')

    setUpOpenshift4ImageRegistryCA
    createImageRegistryPullSecret "${IMAGE_REGISTRY_HOST}"

    imageTool="podman"
    ${imageTool} login -u kubeadmin -p $(oc whoami -t) "${IMAGE_REGISTRY_HOST}" --tls-verify=false

    if [ -z "${CATALOG_SOURCE_IMAGE_NAME}" ]; then
      CATALOG_SOURCE_IMAGE_NAME="operator-catalog-source:0.0.1"
    fi

    if [ -z "${CATALOG_SOURCE_IMAGE}" ]; then
      CATALOG_SOURCE_IMAGE="${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_SOURCE_IMAGE_NAME}"  
    fi

    CATALOG_BUNDLE_IMAGE_NAME="che_operator_bundle:0.0.1"
    CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_BUNDLE_IMAGE_NAME}"

    echo "[INFO] Build bundle image... ${CATALOG_BUNDLE_IMAGE}"
    buildBundleImage "${CATALOG_BUNDLE_IMAGE}" "${imageTool}"

    echo "[INFO] Build catalog image... ${CATALOG_BUNDLE_IMAGE}"
    buildCatalogImage "${CATALOG_SOURCE_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "${imageTool}"

    # For some reason CRC external registry exposed is not working. I'll use the internal registry in cluster which is:image-registry.openshift-image-registry.svc:5000
    CATALOG_SOURCE_IMAGE="image-registry.openshift-image-registry.svc:5000/${NAMESPACE}/${CATALOG_SOURCE_IMAGE_NAME}"
    export CATALOG_SOURCE_IMAGE
    echo "[INFO]: Successfully added catalog source and bundle images to crc image registry: ${CATALOG_SOURCE_IMAGE}"
  else
    echo "[ERROR]: Error to start olm tests. Invalid Platform"
    printHelp
    exit 1
  fi
}

run() {
  createNamespace
  forcePullingOlmImages "${CATALOG_BUNDLE_IMAGE}"
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
