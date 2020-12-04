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
#
# Scripts to prepare OLM(operator lifecycle manager) and install che-operator package
# with specific version using OLM.

BASE_DIR=$(dirname $(dirname $(readlink -f "${BASH_SOURCE[0]}")))/olm
ROOT_DIR=$(dirname "${BASE_DIR}")

source ${ROOT_DIR}/olm/check-yq.sh

SOURCE_INSTALL=$4

if [ -z ${SOURCE_INSTALL} ]; then SOURCE_INSTALL="Marketplace"; fi

platform=$1
if [ "${platform}" == "" ]; then
  echo "Please specify platform ('openshift' or 'kubernetes') as the first argument."
  echo ""
  echo "testUpdate.sh <platform> [<channel>] [<namespace>]"
  exit 1
fi

PACKAGE_VERSION=$2
if [ "${PACKAGE_VERSION}" == "" ]; then
  echo "Please specify PACKAGE_VERSION version"
  exit 1
fi

namespace=$3
if [ "${namespace}" == "" ]; then
  namespace="eclipse-che-preview-test"
fi

channel="stable"
if [[ "${PACKAGE_VERSION}" =~ "nightly" ]]
then
  channel="nightly"
  OPM_BUNDLE_DIR="${ROOT_DIR}/deploy/olm-catalog/eclipse-che-preview-${platform}"
  OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
fi

packageName=eclipse-che-preview-${platform}
if [ "${channel}" == 'nightly' ]; then
  CSV_FILE="${ROOT_DIR}/deploy/olm-catalog/eclipse-che-preview-${platform}/manifests/che-operator.clusterserviceversion.yaml"
else
  if [ ${SOURCE_INSTALL} == "catalog" ]; then
    echo "[ERROR] Stable preview channel doesn't support installation using 'catalog'. Use 'Marketplace' instead of it."
    exit 1
  fi

  platformPath="${BASE_DIR}/${packageName}"
  packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  CSV_FILE="${packageFolderPath}/${PACKAGE_VERSION}/${packageName}.v${PACKAGE_VERSION}.clusterserviceversion.yaml"
fi

CSV=$(yq -r ".metadata.name" "${CSV_FILE}")

echo -e "\u001b[32m PACKAGE_VERSION=${PACKAGE_VERSION} \u001b[0m"
echo -e "\u001b[32m CSV=${CSV} \u001b[0m"
echo -e "\u001b[32m Channel=${channel} \u001b[0m"
echo -e "\u001b[32m Namespace=${namespace} \u001b[0m"

# We don't need to delete ${namespace} anymore since tls secret is precreated there.
# if kubectl get namespace "${namespace}" >/dev/null 2>&1
# then
#   echo "You should delete namespace '${namespace}' before running the update test first."
#   exit 1
# fi

catalog_source() {
    marketplaceNamespace=${namespace};
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
NAMESPACE=${1}
indexIp=${2}
cat <<EOF | oc apply -n "${NAMESPACE}" -f - || return $?
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${packageName}
spec:
  address: "${indexIp}:50051"
  displayName: "Serverless Operator"
  publisher: Red Hat
  sourceType: grpc
EOF
}

applyCheOperatorInstallationSource() {
  if [ ${SOURCE_INSTALL} == "catalog" ]; then
    echo "[INFO] Use catalog source(index) image"
    catalog_source
  else
    if [ "${APPLICATION_REGISTRY}" == "" ]; then
      echo "[INFO] Use default Eclipse Che application registry"
      cat "${platformPath}/operator-source.yaml"
      kubectl apply -f "${platformPath}/operator-source.yaml"
    else
      echo "[INFO] Use custom Che application registry"
      cat "${platformPath}/operator-source.yaml" | \
      sed  -e "s/registryNamespace:.*$/registryNamespace: \"${APPLICATION_REGISTRY}\"/" | \
      kubectl apply -f -
    fi
  fi
}

buildBundleImage() {
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL=${1}
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "Please specify first argument: opm bundle image"
    exit 1
  fi

  imageTool=${2:-docker}

  pushd "${OPM_BUNDLE_DIR}" || exit

  echo "[INFO] build bundle image for dir: ${OPM_BUNDLE_MANIFESTS_DIR}"

  ${OPM_BINARY} alpha bundle build \
    -d "${OPM_BUNDLE_MANIFESTS_DIR}" \
    --tag "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --package "eclipse-che-preview-${platform}" \
    --channels "nightly" \
    --default "nightly" \
    --image-builder "${imageTool}"

  # ${OPM_BINARY} alpha bundle validate -t "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" --image-builder "${imageTool}"

  SKIP_TLS_VERIFY=""
  if [ "${imageTool}" == "podman" ]; then
    SKIP_TLS_VERIFY=" --tls-verify=false"
  fi
  eval "${imageTool}" push "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" "${SKIP_TLS_VERIFY}"

  popd || exit
}

# Build catalog source image with index based on bundle image.
buildCatalogImage() {
  CATALOG_IMAGENAME=${1}
  if [ -z "${CATALOG_IMAGENAME}" ]; then
    echo "Please specify first argument: catalog image"
    exit 1
  fi

  CATALOG_BUNDLE_IMAGE_NAME_LOCAL=${2}
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "Please specify second argument: opm bundle image"
    exit 1
  fi

  imageTool=${3:-docker}

  FROM_INDEX=${4:-""}
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

  eval "${OPM_BINARY}" index add \
       --bundles "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
       --tag "${CATALOG_IMAGENAME}" \
       --pull-tool "${imageTool}" \
       --build-tool "${imageTool}" \
       --binary-image=quay.io/operator-framework/upstream-opm-builder:v1.15.1 \
       --mode semver \
       "${BUILD_INDEX_IMAGE_ARG}" "${SKIP_TLS_ARG}"

  eval "${imageTool}" push "${CATALOG_IMAGENAME}" "${SKIP_TLS_VERIFY}"
}

# HACK. Unfortunately catalog source image bundle job has image pull policy "IfNotPresent".
# It makes troubles for test scripts, because image bundle could be outdated with
# such pull policy. That's why we launch job to fource image bundle pulling before Che installation.
forcePullingOlmImages() {
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL=${1}
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "Please specify first argument: opm bundle image"
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
    popd || exit
  fi
}

createNamespace() {
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
EOF
}

installOperatorMarketPlace() {
  echo "Installing test pre-requisistes"

  marketplaceNamespace="marketplace"
  if [ "${platform}" == "openshift" ];
  then
    marketplaceNamespace="openshift-marketplace";
    applyCheOperatorInstallationSource
  else
      IFS=$'\n' read -d '' -r -a olmApiGroups < <( kubectl api-resources --api-group=operators.coreos.com -o name ) || true
    if [ -z "${olmApiGroups[*]}" ]; then
      OLM_VERSION=0.15.1
      MARKETPLACE_VERSION=4.5
      OPERATOR_MARKETPLACE_VERSION="release-${MARKETPLACE_VERSION}"
      curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/install.sh | bash -s ${OLM_VERSION}
      kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/01_namespace.yaml
      kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/03_operatorsource.crd.yaml
      kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/04_service_account.yaml
      kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/05_role.yaml
      kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/06_role_binding.yaml
      sleep 1
      kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/07_upstream_operatorsource.cr.yaml
      curl -sL https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/08_operator.yaml | \
      sed -e "s;quay.io/openshift/origin-operator-marketplace:latest;quay.io/openshift/origin-operator-marketplace:${MARKETPLACE_VERSION};" | \
      kubectl apply -f -
    fi

    applyCheOperatorInstallationSource

    i=0
    while [ $i -le 240 ]
    do
      if kubectl get catalogsource/"${packageName}" -n "${marketplaceNamespace}"  >/dev/null 2>&1
      then
        break
      fi
      sleep 1
      ((i++))
    done

    if [ $i -gt 240 ]
    then
      echo "Catalog source not created after 4 minutes"
      exit 1
    fi
    if [ "${SOURCE_INSTALL}" == "Marketplace" ]; then
      kubectl get catalogsource/"${packageName}" -n "${marketplaceNamespace}" -o json | jq '.metadata.namespace = "olm" | del(.metadata.creationTimestamp) | del(.metadata.uid) | del(.metadata.resourceVersion) | del(.metadata.generation) | del(.metadata.selfLink) | del(.status)' | kubectl apply -f -
      marketplaceNamespace="olm"
    fi
  fi
}

subscribeToInstallation() {
  CSV_NAME="${1-${CSV}}"

  echo "Subscribing to version: ${CSV_NAME}"

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
  sourceNamespace: ${marketplaceNamespace}
  startingCSV: ${CSV_NAME}
EOF

  kubectl describe subscription/"${packageName}" -n "${namespace}"

  kubectl wait subscription/"${packageName}" -n "${namespace}" --for=condition=InstallPlanPending --timeout=240s
  if [ $? -ne 0 ]
  then
    echo Subscription failed to install the operator
    exit 1
  fi

  kubectl describe subscription/"${packageName}" -n "${namespace}"
}

installPackage() {
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

applyCRCheCluster() {
  echo "Creating Custom Resource"
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${CSV_FILE}")
  CR=$(echo "$CRs" | yq -r ".[0]")
  if [ "${platform}" == "kubernetes" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi
  if [ "${platform}" == "openshift" ] && [ "${OAUTH}" == "false" ]; then
    CR=$(echo "$CR" | yq -r ".spec.auth.openShiftoAuth = false")
  fi

  echo "$CR" | kubectl apply -n "${namespace}" -f -
}

waitCheServerDeploy() {
  echo "Waiting for Che server to be deployed"
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
    echo "Che server did't start after 8 minutes"
    exit 1
  fi
}

getBundleListFromCatalogSource() {
  CATALOG_POD=$(kubectl get pods -n ${namespace} -o yaml | yq -r ".items[] | select(.metadata.name | startswith(\"eclipse-che-preview-${platform}\")) | .metadata.name")
  kubectl wait --for=condition=ready "pods/${CATALOG_POD}" --timeout=60s -n "${namespace}"

  CATALOG_SERVICE=$(kubectl get service "eclipse-che-preview-${platform}" -n "${namespace}" -o yaml)
  CATALOG_IP=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.clusterIP")
  CATALOG_PORT=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.ports[0].targetPort")

  LIST_BUNDLES=$(kubectl run --generator=run-pod/v1 grpcurl-query -n che \
  --rm=true \
  --restart=Never \
  --attach=true \
  --image=docker.io/fullstorydev/grpcurl:v1.7.0 \
  --  -plaintext "${CATALOG_IP}:${CATALOG_PORT}" api.Registry.ListBundles
  )

  LIST_BUNDLES=$(echo "${LIST_BUNDLES}" | head -n -1)

  echo "${LIST_BUNDLES}"
}

getPreviousCSVInfo() {
  previousBundle=$(echo "${LIST_BUNDLES}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 2]')
  PREVIOUS_CSV_NAME=$(echo "${previousBundle}" | yq -r ".csvName")
  if [ "${PREVIOUS_CSV_NAME}" == "null" ]; then
    echo "Error: bundle hasn't go previous bundle."
    exit 1
  fi
  export PREVIOUS_CSV_NAME
  PREVIOUS_CSV_BUNDLE_IMAGE=$(echo "${previousBundle}" | yq -r ".bundlePath")
  export PREVIOUS_CSV_BUNDLE_IMAGE
}

getLatestCSVInfo() {
  latestBundle=$(echo "${LIST_BUNDLES}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 1]')
  LATEST_CSV_NAME=$(echo "${latestBundle}" | yq -r ".csvName")
  export LATEST_CSV_NAME
  LATEST_CSV_BUNDLE_IMAGE=$(echo "${latestBundle}" | yq -r ".bundlePath")
  export LATEST_CSV_BUNDLE_IMAGE
}
