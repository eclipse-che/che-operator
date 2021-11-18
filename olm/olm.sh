#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# Scripts to prepare OLM(operator lifecycle manager) and install che-operator package
# with specific version using OLM.

BASE_DIR=$(dirname $(readlink -f "${BASH_SOURCE[0]}"))
ROOT_DIR=$(dirname "${BASE_DIR}")

source ${ROOT_DIR}/olm/check-yq.sh

function getPackageName() {
  platform="${1}"
  if [ -z "${1}" ]; then
      echo "[ERROR] Please specify first argument: 'platform'"
      exit 1
  fi

  echo "eclipse-che-preview-${platform}"
}

function getBundlePath() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  channel="${2}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] Please specify second argument: 'channel'"
    exit 1
  fi

  echo "${ROOT_DIR}/bundle/${channel}/$(getPackageName "${platform}")"
}

createCatalogSource() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  namespace="${2}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify second argument: 'namespace'"
    exit 1
  fi
  CATALOG_IMAGENAME="${3}"
  if [ -z "${CATALOG_IMAGENAME}" ]; then
    echo "[ERROR] Please specify third argument: 'catalog image'"
    exit 1
  fi
  packageName=$(getPackageName "${platform}")

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  sourceType: grpc
  image: ${CATALOG_IMAGENAME}
  updateStrategy:
    registryPoll:
      interval: 5m
EOF
}

# Create catalog source to communicate with OLM using google rpc protocol.
createRpcCatalogSource() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  namespace="${2}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify second argument: 'namespace'"
    exit 1
  fi
  indexIP="${3}"
  if [ -z "${indexIP}" ]; then
    echo "[ERROR] Please specify third argument: 'index IP'"
    exit 1
  fi

  packageName=$(getPackageName "${platform}")

cat <<EOF | oc apply -n "${namespace}" -f - || return $?
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${packageName}
spec:
  address: "${indexIP}:50051"
  displayName: "Serverless Operator"
  publisher: Red Hat
  sourceType: grpc
EOF
}

buildBundleImage() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${2}"
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "[ERROR] Please specify second argument: 'opm bundle'"
    exit 1
  fi
  channel="${3}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] Please specify third argument: 'channel'"
    exit 1
  fi
  imageTool="${4}"
  if [ -z "${imageTool}" ]; then
    echo "[ERROR] Please specify fourth argument: 'image tool'"
    exit 1
  fi

  echo "[INFO] build bundle image"

  pushd "${ROOT_DIR}" || exit

  make bundle-build bundle-push channel="${channel}" BUNDLE_IMG="${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" platform="${platform}" IMAGE_TOOL="${imageTool}"
  popd || exit
}

# Build catalog source image with index based on bundle image.
buildCatalogImage() {
  CATALOG_IMAGENAME="${1}"
  if [ -z "${CATALOG_IMAGENAME}" ]; then
    echo "[ERROR] Please specify first argument: 'catalog image'"
    exit 1
  fi

  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${2}"
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "[ERROR] Please specify second argument: 'opm bundle image'"
    exit 1
  fi

  imageTool="${3}"
  if [ -z "${imageTool}" ]; then
    echo "[ERROR] Please specify third argument: 'image tool'"
    exit 1
  fi

  forceBuildAndPush="${4}"
  if [ -z "${forceBuildAndPush}" ]; then
    echo "[ERROR] Please specify fourth argument: 'force build and push: true or false'"
    exit 1
  fi

  # optional argument
  FROM_INDEX=${5:-""}
  BUILD_INDEX_IMAGE_ARG=""
  if [ ! "${FROM_INDEX}" == "" ]; then
    BUILD_INDEX_IMAGE_ARG=" --from-index ${FROM_INDEX}"
  fi

  SKIP_TLS_ARG=""
  SKIP_TLS_VERIFY=""
  if [ "${imageTool}" == "podman" ]; then
    SKIP_TLS_ARG=" --skip-tls"
    SKIP_TLS_VERIFY=" --tls-verify=false"
  fi

  pushd "${ROOT_DIR}" || exit

  INDEX_ADD_CMD="make catalog-build \
    CATALOG_IMG=\"${CATALOG_IMAGENAME}\" \
    BUNDLE_IMG=\"${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}\" \
    IMAGE_TOOL=\"${imageTool}\" \
    FROM_INDEX_OPT=\"${BUILD_INDEX_IMAGE_ARG}\""

  exitCode=0
  # Execute command and store an error output to the variable for following handling.
  {
    output="$(eval "${INDEX_ADD_CMD}" 2>&1 1>&3 3>&-)"; } 3>&1 || \
    {
      exitCode="$?";
      echo "[INFO] ${exitCode}";
      true;
    }
    echo "${output}"
  if [[ "${output}" == *"already exists, Bundle already added that provides package and csv"* ]] && [[ "${forceBuildAndPush}" == "true" ]]; then
    echo "[INFO] Ignore error 'Bundle already added'"
    # Catalog bundle image contains bundle reference, continue without unnecessary push operation
    return
  else
    echo "[INFO] ${exitCode}"
    if [ "${exitCode}" != 0 ]; then
      exit "${exitCode}"
    fi
  fi

  make catalog-push CATALOG_IMG="${CATALOG_IMAGENAME}"

  popd || exit
}

# HACK. Unfortunately catalog source image bundle job has image pull policy "IfNotPresent".
# It makes troubles for test scripts, because image bundle could be outdated with
# such pull policy. That's why we launch job to fource image bundle pulling before Che installation.
forcePullingOlmImages() {
  namespace="${1}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify first argument: 'namespace'"
    exit 1
  fi
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${2}"
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "[ERROR] Please specify second argument: opm bundle image"
    exit 1
  fi

  yq -r "(.spec.template.spec.containers[0].image) = \"${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}\"" "${BASE_DIR}/force-pulling-olm-images-job.yaml" | kubectl apply -f - -n "${namespace}"

  kubectl wait --for=condition=complete --timeout=30s job/force-pulling-olm-images-job -n "${namespace}"

  kubectl delete job/force-pulling-olm-images-job -n "${namespace}"
}

installOPM() {
  OPM_BINARY=$(command -v opm) || true
  if [[ ! -x $OPM_BINARY ]]; then
    OPM_TEMP_DIR="$(mktemp -q -d -t "OPM_XXXXXX" 2>/dev/null || mktemp -q -d)"
    pushd "${OPM_TEMP_DIR}" || exit

    echo "[INFO] Downloading 'opm' cli tool..."
    curl -sLo opm "$(curl -sL https://api.github.com/repos/operator-framework/operator-registry/releases/33432389 | jq -r '[.assets[] | select(.name == "linux-amd64-opm")] | first | .browser_download_url')"
    export OPM_BINARY="${OPM_TEMP_DIR}/opm"
    chmod +x "${OPM_BINARY}"
    echo "[INFO] Downloading completed!"
    echo "[INFO] 'opm' binary path: ${OPM_BINARY}"
    ${OPM_BINARY} version
    popd || exit
  fi
}

createNamespace() {
  namespace="${1}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify first argument: 'namespace'"
    exit 1
  fi

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
EOF
}

installOperatorMarketPlace() {
  echo "[INFO] Installing test pre-requisistes"
  IFS=$'\n' read -d '' -r -a olmApiGroups < <( kubectl api-resources --api-group=operators.coreos.com -o name ) || true
  if [ -z "${olmApiGroups[*]}" ]; then
      OLM_VERSION=v0.17.0
      curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/install.sh | bash -s ${OLM_VERSION}
  fi
}

installCatalogSource() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  namespace="${2}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify second argument: 'namespace'"
    exit 1
  fi
  CATALOG_IMAGENAME=${3}
  if [ -z "${CATALOG_IMAGENAME}" ]; then
    echo "[ERROR] Please specify third argument: 'catalog image'"
    exit 1
  fi
  packageName=$(getPackageName "${platform}")

  createCatalogSource "${platform}" "${namespace}" "${CATALOG_IMAGENAME}"

  i=0
  while [ $i -le 240 ]
  do
    if kubectl get catalogsource/"${packageName}" -n "${namespace}"  >/dev/null 2>&1
    then
      break
    fi
    sleep 1
    ((i++))
  done

  if [ $i -gt 240 ]
  then
    echo "[ERROR] Catalog source not created after 4 minutes"
    exit 1
  fi
}

subscribeToInstallation() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  namespace="${2}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify second argument: 'namespace'"
    exit 1
  fi
  channel="${3}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] Please specify third argument: 'channel'"
    exit 1
  fi

  # fourth argument is an optional
  CSV_NAME="${4-${CSV_NAME}}"
  if [ -n "${CSV_NAME}" ]; then
    echo "[INFO] Subscribing to the version: '${CSV_NAME}'"
  else
    echo "[INFO] Subscribing to latest version for channel: '${channel}'"
  fi

  packageName=$(getPackageName "${platform}")

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: ${namespace}
spec:
  targetNamespaces:
  - ${namespace}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  channel: ${channel}
  installPlanApproval: Manual
  name: ${packageName}
  source: ${packageName}
  sourceNamespace: ${namespace}
  startingCSV: ${CSV_NAME}
EOF

  kubectl describe subscription/"${packageName}" -n "${namespace}"

  kubectl wait subscription/"${packageName}" -n "${namespace}" --for=condition=InstallPlanPending --timeout=240s
  if [ $? -ne 0 ]
  then
    echo "[ERROR] Subscription failed to install the operator"
    exit 1
  fi

  kubectl describe subscription/"${packageName}" -n "${namespace}"
}

installPackage() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  namespace="${2}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify second argument: 'namespace'"
    exit 1
  fi
  packageName=$(getPackageName "${platform}")

  echo "[INFO] Install operator package ${packageName} into namespace ${namespace}"
  installPlan=$(kubectl get subscription/"${packageName}" -n "${namespace}" -o jsonpath='{.status.installplan.name}')

  kubectl patch installplan/"${installPlan}" -n "${namespace}" --type=merge -p '{"spec":{"approved":true}}'

  kubectl wait installplan/"${installPlan}" -n "${namespace}" --for=condition=Installed --timeout=240s
  if [ $? -ne 0 ]
  then
    echo InstallPlan failed to install the operator
    exit 1
  fi
}

applyCheClusterCR() {
  CSV_NAME=${1}
  PLATFORM=${2}

  CHECLUSTER=$(kubectl get csv ${CSV_NAME} -n ${NAMESPACE} -o yaml \
    | yq -r ".metadata.annotations[\"alm-examples\"] | fromjson | .[] | select(.kind == \"CheCluster\")" \
    | yq -r ".spec.devWorkspace.enable = ${DEV_WORKSPACE_ENABLE:-false}" \
    | yq -r ".spec.server.serverExposureStrategy = \"${CHE_EXPOSURE_STRATEGY:-multi-host}\"" \
    | yq -r ".spec.imagePuller.enable = ${IMAGE_PULLER_ENABLE:-false}")

  echo "${CHECLUSTER}"
  if [[ ${PLATFORM} == "kubernetes" ]]; then
    CHECLUSTER=$(echo "${CHECLUSTER}" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi

  echo "[INFO] Creating Custom Resource: "
  echo "${CHECLUSTER}"

  echo "${CHECLUSTER}" | kubectl apply -n $NAMESPACE -f -
}

waitCheServerDeploy() {
  namespace="${1}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify first argument: 'namespace'"
    exit 1
  fi

  echo "[INFO] Waiting for Che server to be deployed"
  set +e -x

  i=0
  while [[ $i -le 480 ]]
  do
    status=$(kubectl get checluster/eclipse-che -n "${namespace}" -o jsonpath={.status.cheClusterRunning})
    kubectl get pods -n "${namespace}"
    if [ "${status:-UNAVAILABLE}" == "Available" ]
    then
      break
    fi
    sleep 10
    ((i++))
  done

  if [ $i -gt 480 ]
  then
    echo "[ERROR] Che server did't start after 8 minutes"
    exit 1
  fi
}

waitCatalogSourcePod() {
  sleep 10s

  CURRENT_TIME=$(date +%s)
  ENDTIME=$(($CURRENT_TIME + 300))
  CATALOG_POD=$(kubectl get pods -n "${namespace}" -o yaml | yq -r ".items[] | select(.metadata.name | startswith(\"${packageName}\")) | .metadata.name")
  while [ $(date +%s) -lt $ENDTIME ]; do
    if [[ -z "$CATALOG_POD" ]]
    then
        CATALOG_POD=$(kubectl get pods -n "${namespace}" -o yaml | yq -r ".items[] | select(.metadata.name | startswith(\"${packageName}\")) | .metadata.name")
        sleep 10
    else
        kubectl wait --for=condition=ready pod/"${CATALOG_POD}" -n "${namespace}" --timeout=180s
        break
    fi
  done
}

getBundleListFromCatalogSource() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] Please specify first argument: 'platform'"
    exit 1
  fi
  namespace="${2}"
  if [ -z "${namespace}" ]; then
    echo "[ERROR] Please specify second argument: 'namespace'"
    exit 1
  fi
  packageName=$(getPackageName "${platform}")
  # Wait until catalog pod is created in cluster
  waitCatalogSourcePod

  CATALOG_SERVICE=$(kubectl get service "${packageName}" -n "${namespace}" -o yaml)
  CATALOG_IP=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.clusterIP")
  CATALOG_PORT=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.ports[0].targetPort")

  LIST_BUNDLES=$(kubectl run grpcurl-query -n "${namespace}" \
  --rm=true \
  --restart=Never \
  --attach=true \
  --image=docker.io/fullstorydev/grpcurl:v1.7.0 \
  --  -plaintext "${CATALOG_IP}:${CATALOG_PORT}" api.Registry.ListBundles
  )

  LIST_BUNDLES=$(echo "${LIST_BUNDLES}" | head -n -1)
}

getPreviousCSVInfo() {
  channel="${1}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] Please specify first argument: 'channel'"
    exit 1
  fi

  previousBundle=$(echo "${LIST_BUNDLES}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 2]')
  PREVIOUS_CSV_NAME=$(echo "${previousBundle}" | yq -r ".csvName")
  if [ "${PREVIOUS_CSV_NAME}" == "null" ]; then
    echo "[ERROR] Catalog source image hasn't got previous bundle."
    exit 1
  fi
  export PREVIOUS_CSV_NAME
  PREVIOUS_CSV_BUNDLE_IMAGE=$(echo "${previousBundle}" | yq -r ".bundlePath")
  export PREVIOUS_CSV_BUNDLE_IMAGE
}

getLatestCSVInfo() {
  channel="${1}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] Please specify first argument: 'channel'"
    exit 1
  fi

  latestBundle=$(echo "${LIST_BUNDLES}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 1]')
  LATEST_CSV_NAME=$(echo "${latestBundle}" | yq -r ".csvName")
  export LATEST_CSV_NAME
  LATEST_CSV_BUNDLE_IMAGE=$(echo "${latestBundle}" | yq -r ".bundlePath")
  export LATEST_CSV_BUNDLE_IMAGE
}
