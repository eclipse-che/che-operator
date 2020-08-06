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

if [ -z "${BASE_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")" && pwd)
fi

echo "${BASE_DIR}"
SCRIPT=$(readlink -f "$0")
echo "[INFO] ${SCRIPT}"
SCRIPT_DIR=$(dirname "$SCRIPT");
echo "[INFO] ${SCRIPT_DIR}"

source ${BASE_DIR}/check-yq.sh

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
fi

packageName=eclipse-che-preview-${platform}
platformPath=${BASE_DIR}/${packageName}
if [ -z "${packageFolderPath}" ]; then
  packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
fi

# Todo check, maybe it's unused...
# packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
# CSV="eclipse-che-preview-${platform}.${PACKAGE_VERSION}"

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

checkImagePushTridentionals() {
  if [ -z "${QUAY_USERNAME}" ] || [ -z "${QUAY_PASSWORD}" ]; then
    echo "[ERROR] Should be defined env variables QUAY_USERNAME, QUAY_PASSWORD"
    exit 1
  fi
}

pushImage() {
  checkImagePushTridentionals

  imageName=$1
  if [ -z "${imageName}" ]; then
    echo "Please specify first argument: imageName"
    exit 1
  fi

  docker push "${imageName}"
}

catalog_source() {
  echo "--- Use default eclipse che application registry ---"
  if [ ${SOURCE_INSTALL} == "LocalCatalog" ]; then
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
  else
    cat "${platformPath}/operator-source.yaml"
    kubectl apply -f "${platformPath}/operator-source.yaml"
  fi
}

# do it only when it's required or remove using deprecated operator source
applyCheOperatorSource() {
  # echo "Apply che-operator source"
  if [ "${APPLICATION_REGISTRY}" == "" ]; then
    catalog_source
  else
    echo "---- Use non default application registry ${APPLICATION_REGISTRY} ---"

    cat "${platformPath}/operator-source.yaml" | \
    sed  -e "s/registryNamespace:.*$/registryNamespace: \"${APPLICATION_REGISTRY}\"/" | \
    kubectl apply -f -
  fi
}

loginToImageRegistry() {
  docker login -u "${QUAY_USERNAME}" -p "${QUAY_PASSWORD}" "quay.io"
}

buildBundleImage() {
  # checkImagePushTridentionals

  OPM_BUNDLE_MANIFESTS_DIR=$1
  if [ -z "${OPM_BUNDLE_MANIFESTS_DIR}" ]; then
    echo "Please specify first argument: opm bundle manifest directory"
    exit 1
  fi

  CATALOG_BUNDLE_IMAGE_NAME_LOCAL=${2}
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "Please specify second argument: opm bundle image"
    exit 1
  fi

  pushd "${OPM_BUNDLE_DIR}" || exit

  echo "[INFO] build bundle image for dir: ${OPM_BUNDLE_DIR}"

  ${OPM_BINARY} alpha bundle build \
    -d "${OPM_BUNDLE_MANIFESTS_DIR}" \
    --tag "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --package "eclipse-che-preview-${platform}" \
    --channels "stable,nightly" \
    --default "stable" \
    --image-builder docker

  ${OPM_BINARY} alpha bundle validate -t "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" 

  docker push "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  popd || exit
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

  yq -r "(.spec.template.spec.containers[0].image) = \"${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}\"" "${SCRIPT_DIR}/force-pulling-olm-images-job.yaml" | kubectl apply -f - -n "${namespace}"

  kubectl wait --for=condition=complete --timeout=30s job/force-pulling-olm-images-job -n "${namespace}"

  kubectl delete job/force-pulling-olm-images-job -n "${namespace}"
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

  FROM_INDEX=${3}
  if [ -n "${FROM_INDEX}" ]; then
    BUILD_INDEX_IMAGE_ARG=" --from-index ${FROM_INDEX}"
  fi

  eval "${OPM_BINARY}" index add --bundles "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
       --tag "${CATALOG_IMAGENAME}" \
       --build-tool docker \
       --mode semver "${BUILD_INDEX_IMAGE_ARG}"
  # --skip-tls # local registry launched without https

  docker push "${CATALOG_IMAGENAME}"
}

installOPM() {
  OPM_BINARY=$(command -v opm) || true
  if [[ ! -x $OPM_BINARY ]]; then
    OPM_TEMP_DIR="$(mktemp -q -d -t "OPM_XXXXXX" 2>/dev/null || mktemp -q -d)"
    pushd "${OPM_TEMP_DIR}" || exit

    echo "[INFO] Downloading 'opm' cli tool..."
    curl -sLo opm "$(curl -sL https://api.github.com/repos/operator-framework/operator-registry/releases/28130850 | jq -r '[.assets[] | select(.name == "linux-amd64-opm")] | first | .browser_download_url')"
    export OPM_BINARY="${OPM_TEMP_DIR}/opm"
    chmod +x "${OPM_BINARY}"
    echo "[INFO] Downloading completed!"
    popd || exit
  fi
  echo "[INFO] 'opm' binary path: ${OPM_BINARY}"
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
    applyCheOperatorSource
  else
    IFS=$'\n' read -d '' -r -a olmApiGroups < <( kubectl api-resources --api-group=operators.coreos.com -o name ) || true
    if [ -z "${olmApiGroups[*]}" ]; then
      curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.15.1/install.sh -o install.sh
      chmod +x install.sh
      ./install.sh 0.15.1
      rm -rf install.sh
      echo "Done"
    fi

    applyCheOperatorSource

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

  echo "Subscribing to version: ${CSV}"

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
EOF

# startingCSV: eclipse-che-preview-kubernetes.v7.16.2-0.nightly

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

  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/${PACKAGE_VERSION}/manifests/${packageName}.${PACKAGE_VERSION}.clusterserviceversion.yaml")
  CR=$(echo "$CRs" | yq -r ".[0]")
  if [ "${platform}" == "kubernetes" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi

  echo "$CR" | kubectl apply -n "${namespace}" -f -
}

waitCheServerDeploy() {
  echo "Waiting for Che server to be deployed"

  i=0
  while [ $i -le 480 ]
  do
    status=$(kubectl get checluster/eclipse-che -n "${namespace}" -o jsonpath={.status.cheClusterRunning})
    if [ "${status}" == "Available" ]
    then
      break
    fi
    sleep 1
    ((i++))
  done

  if [ $i -gt 480 ]
  then
    echo "Che server did't start after 8 minutes"
    exit 1
  fi
}
