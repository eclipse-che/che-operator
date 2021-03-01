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
	echo '    CATALOG_SOURCE_IMAGE     - Image name used to create a catalog source in cluster'
  echo ''
  echo 'EXAMPLE of running: ${OPERATOR_REPO}/olm/testCatalogSource.sh openshift nightly che my_image_name'
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

# Assign catalog source image
CATALOG_SOURCE_IMAGE=$4

IMAGE_REGISTRY_USER_NAME=${IMAGE_REGISTRY_USER_NAME:-eclipse}
echo "[INFO] Image 'IMAGE_REGISTRY_USER_NAME': ${IMAGE_REGISTRY_USER_NAME}"

init() {
  source "${OLM_DIR}/olm.sh"
  OPM_BUNDLE_DIR=$(getBundlePath "${PLATFORM}" "${CHANNEL}")

  CSV_FILE="${OPM_BUNDLE_DIR}/manifests/che-operator.clusterserviceversion.yaml"
  CSV_NAME=$(yq -r ".metadata.name" "${CSV_FILE}")

  installOPM
}

buildOLMImages() {
  # Manage catalog source for every platform in part.
  # 1. Kubernetes:
  #    a) Use Minikube cluster. Enable registry addon, build catalog source and olm bundle images, push them to embedded private registry.
  #    b) Provide image registry env variables to push images to the real public registry(docker.io, quay.io etc).
  # 2. Openshift: build bundle image and push it using image stream. Launch deployment with custom grpc based catalog source image to install the latest bundle.
  if [[ "${PLATFORM}" == "kubernetes" ]]
  then
    echo "[INFO]: Kubernetes platform detected"

    # Build operator image
    if [ -n "${OPERATOR_IMAGE}" ];then
      echo "[INFO]: Build operator image ${OPERATOR_IMAGE}..."
      pushd "${OPERATOR_REPO}" || true
      docker build --no-cache -t "${OPERATOR_IMAGE}" -f Dockerfile .
      docker push "${OPERATOR_IMAGE}"
      echo "${OPERATOR_IMAGE}"
      popd || true

      # Use operator image in the latest CSV
      sed -i "s|image: quay.io/eclipse/che-operator:nightly|image: ${OPERATOR_IMAGE}|" "${CSV_FILE}"
      sed -i 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|' "${CSV_FILE}"
    fi

    CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che_operator_bundle:0.0.1"
    CATALOG_SOURCE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/testing_catalog:0.0.1"

    echo "[INFO] Build bundle image... ${CATALOG_BUNDLE_IMAGE}"
    buildBundleImage "${PLATFORM}" "${CATALOG_BUNDLE_IMAGE}" "${CHANNEL}" "docker"

    echo "[INFO] Build catalog image... ${CATALOG_BUNDLE_IMAGE}"
    buildCatalogImage "${CATALOG_SOURCE_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker" "false"

    echo "[INFO]: Successfully created catalog source container image and enabled minikube ingress."
  elif [[ "${PLATFORM}" == "openshift" ]]
  then
    echo "[INFO]: Starting to build catalog image and push to ImageStream."

    echo "============"
    echo "[INFO] Current user is $(oc whoami)"
    echo "============"

    oc new-project "${NAMESPACE}" || true

    pull_user="puller"
    pull_password="puller"
    add_user "${pull_user}" "${pull_password}"

    if [ -z "${KUBECONFIG}" ]; then
      KUBECONFIG="${HOME}/.kube/config"
    fi
    TEMP_KUBE_CONFIG="/tmp/$pull_user.kubeconfig"
    rm -rf "${TEMP_KUBE_CONFIG}"
    cp "${KUBECONFIG}" "${TEMP_KUBE_CONFIG}"
    sleep 180

    loginLogFile="/tmp/login-log"
    touch "${loginLogFile}"
    loginCMD="oc login --kubeconfig=${TEMP_KUBE_CONFIG}  --username=${pull_user} --password=${pull_password} > ${loginLogFile}"
    timeout 900 bash -c "${loginCMD}" || echo "[ERROR] Login Fail"
    echo "[INFO] $(cat "${loginLogFile}" || true)"

    echo "[INFO] Applying policy registry-viewer to user '${pull_user}'..."
    oc -n "$NAMESPACE" policy add-role-to-user registry-viewer "$pull_user"

    echo "[INFO] Trying to retrieve user '${pull_user}' token..."
    token=$(oc --kubeconfig=${TEMP_KUBE_CONFIG} whoami -t)
    echo "[INFO] User '${pull_user}' token is: ${token}"

    oc -n "${NAMESPACE}" new-build --binary --strategy=docker --name serverless-bundle

    cp -rf "${OPM_BUNDLE_DIR}/bundle.Dockerfile" "${OPM_BUNDLE_DIR}/Dockerfile"
    if oc -n "${NAMESPACE}" start-build serverless-bundle --from-dir "${OPM_BUNDLE_DIR}"; then
      rm -rf "${OPM_BUNDLE_DIR}/Dockerfile"
    else
      rm -rf "${OPM_BUNDLE_DIR}/Dockerfile"
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
  indexIP="$(oc -n "${NAMESPACE}" get pods -l app=catalog-source-app -o jsonpath='{.items[0].status.podIP}')"

  # Install the catalogsource.
  createRpcCatalogSource "${PLATFORM}" "${NAMESPACE}" "${indexIP}"
  else
    echo "[ERROR]: Error to start olm tests. Invalid Platform"
    printHelp
    exit 1
  fi
}

run() {
  createNamespace "${NAMESPACE}"
  if [ ! "${PLATFORM}" == "openshift" ]; then
    forcePullingOlmImages "${NAMESPACE}" "${CATALOG_BUNDLE_IMAGE}"
  fi

  installOperatorMarketPlace
  if [ ! "${PLATFORM}" == "openshift" ]; then
    installCatalogSource "${PLATFORM}" "${NAMESPACE}" "${CATALOG_SOURCE_IMAGE}"
  fi
  subscribeToInstallation "${PLATFORM}" "${NAMESPACE}" "${CHANNEL}" "${CSV_NAME}"

  installPackage "${PLATFORM}" "${NAMESPACE}"
  applyCRCheCluster "${PLATFORM}" "${NAMESPACE}" "${CSV_FILE}"
  waitCheServerDeploy "${NAMESPACE}"
}

function add_user {
  name=$1
  pass=$2

  echo "[INFO] Creating user $name:$pass"

  PASSWD_TEMP_DIR="$(mktemp -q -d -t "passwd_XXXXXX" 2>/dev/null || mktemp -q -d)"
  HT_PASSWD_FILE="${PASSWD_TEMP_DIR}/users.htpasswd"
  touch "${HT_PASSWD_FILE}"

  htpasswd -b "${HT_PASSWD_FILE}" "$name" "$pass"
  echo "====== HTPASSWD content is:========"
  cat "${HT_PASSWD_FILE}"
  echo "==================================="

  if ! kubectl get secret htpass-secret -n openshift-config 2>/dev/null; then
  kubectl create secret generic htpass-secret \
    --from-file=htpasswd="${HT_PASSWD_FILE}" \
    -n openshift-config
  fi

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
