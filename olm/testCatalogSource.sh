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
    PACKAGE_FOLDER_PATH="${OPERATOR_REPO}/deploy/olm-catalog/che-operator/eclipse-che-preview-${PLATFORM}"
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

    echo "============"
    echo "[INFO] Current user is $(oc whoami)"
    echo "============"
    
    if [[ "${OPENSHIFT_CI}" == "true" ]];then echo "Openshift ci!"; fi

    # CRC_BINARY=$(command -v crc) || true
    # if [[ ! "$(oc whoami  2>/dev/null)" =~ "kube:admin" ]] && [[ ! -x "${CRC_BINARY}" ]; then 
    #   oc login -u kubeadmin -p $(crc console --credentials | awk -F "kubeadmin" '{print $2}' | cut -c 5- | rev | cut -c31- | rev) https://api.crc.testing:6443
    # fi

    oc new-project "${NAMESPACE}" || true

    pull_user="puller"
    pull_password="puller"
    add_user "${pull_user}" "${pull_password}"
    
    if [ -z "${KUBECONFIG}" ]; then
      KUBECONFIG="${HOME}/.kube/config"
    fi
    cp "${KUBECONFIG}" "$pull_user.kubeconfig"
    sleep 180

    loginLogFile="/tmp/login-log"
    touch "${loginLogFile}"
    loginCMD="oc login --kubeconfig=$pull_user.kubeconfig  --username=${pull_user} --password=${pull_password} > ${loginLogFile}"
    timeout 900 bash -c "${loginCMD}" || echo "Login Fail"
    echo "[INFO] $(cat "${loginLogFile}" || true)"

    echo "[INFO] Applying policy registry-viewer to user '${pull_user}'..."
    oc -n "$NAMESPACE" policy add-role-to-user registry-viewer "$pull_user"

    echo "[INFO] Trying to retrieve user '${pull_user}' token..."
    token=$(oc --kubeconfig=$pull_user.kubeconfig whoami -t)
    echo "[INFO] User '${pull_user}' token is: ${token}"

    oc -n "${NAMESPACE}" new-build --binary --strategy=docker --name serverless-bundle

    cp -rf "${PACKAGE_FOLDER_PATH}/bundle.Dockerfile" "${PACKAGE_FOLDER_PATH}/Dockerfile"
    if oc -n "${NAMESPACE}" start-build serverless-bundle --from-dir "${PACKAGE_FOLDER_PATH}"; then
      rm -rf "${PACKAGE_FOLDER_PATH}/Dockerfile"
    else
      rm -rf "${PACKAGE_FOLDER_PATH}/Dockerfile"
      echo "[ERROR ]Failed to build bundle image."
      exit 1
    fi

cat <<EOF | oc apply -n "${NAMESPACE}" -f - || return $?
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog-source-app
spec:
  selector:
    matchLabels:
      app: catalog-source-app
  template:
    metadata:
      labels:
        app: catalog-source-app
    spec:
      containers:
      - name: registry
        image: quay.io/openshift-knative/index
        ports:
        - containerPort: 50051
          name: grpc
          protocol: TCP
        livenessProbe:
          exec:
            command:
            - grpc_health_probe
            - -addr=localhost:50051
        readinessProbe:
          exec:
            command:
            - grpc_health_probe
            - -addr=localhost:50051
        command:
        - /bin/sh
        - -c
        - |-
          podman login -u ${pull_user} -p ${token} image-registry.openshift-image-registry.svc:5000
          /bin/opm registry add --container-tool=podman -d index.db --mode=semver -b image-registry.openshift-image-registry.svc:5000/${NAMESPACE}/serverless-bundle && \
          /bin/opm registry serve -d index.db -p 50051
EOF

  # Wait for the index pod to be up to avoid inconsistencies with the catalog source.
  kubectl wait --for=condition=ready "pods" -l app=catalog-source-app --timeout=120s -n "${NAMESPACE}" || true
  indexip="$(oc -n "$NAMESPACE" get pods -l app=catalog-source-app -o jsonpath='{.items[0].status.podIP}')"

  # Install the catalogsource.
  createRpcCatalogSource "${NAMESPACE}" "${indexip}"
  else
    echo "[ERROR]: Error to start olm tests. Invalid Platform"
    printHelp
    exit 1
  fi
}

run() {
  createNamespace
  if [ ! ${PLATFORM} == "openshift" ]; then
    forcePullingOlmImages "${CATALOG_BUNDLE_IMAGE}"
  fi
  
  installOperatorMarketPlace
  subscribeToInstallation

  installPackage
  applyCRCheCluster
  waitCheServerDeploy
}

function add_user {
  name=$1
  pass=$2

  echo "Creating user $name:$pass"

  PASSWD_TEMP_DIR="$(mktemp -q -d -t "passwd_XXXXXX" 2>/dev/null || mktemp -q -d)"
  HT_PASSWD_FILE="${PASSWD_TEMP_DIR}/users.htpasswd"
  touch "${HT_PASSWD_FILE}"

  htpasswd -b "${HT_PASSWD_FILE}" "$name" "$pass"
  echo "HTPASSWD content is:======================="
  cat "${HT_PASSWD_FILE}"
  echo "==================================="

  kubectl create secret generic htpass-secret \
    --from-file=htpasswd="${HT_PASSWD_FILE}" \
    -n openshift-config

cat <<EOF | oc apply -n "${NAMESPACE}" -f - || return $?
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: my_htpasswd_provider
    mappingMethod: claim
    type: HTPasswd
    htpasswd:
      fileData:
        name: htpass-secret
EOF
}

init
buildOLMImages
run
echo -e "\u001b[32m Done. \u001b[0m"
