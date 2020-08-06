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

set -e

platform=$1
if [ "${platform}" == "" ]; then
  echo "Please specify platform ('openshift' or 'kubernetes') as the first argument."
  echo ""
  echo "testUpdate.sh <platform> [<channel>] [<namespace>]"
  exit 1
fi

channel=$2
if [ "${channel}" == "" ]; then
  channel="nightly"
fi

NAMESPACE=$3

#Check if minikube is installed.
if ! hash minikube 2>/dev/null; then
  echo "Minikube is not installed."
  exit 1
fi

init() {
  #Setting current directory
  BASE_DIR=$(cd "$(dirname "$0")" && pwd)
  ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")

  # Setting The catalog image and the image and tag; and install type
  Install_Type="LocalCatalog"

  # GET the package version to apply
  # packageName=eclipse-che-preview-${platform}
  packageFolderPath="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
  # packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  # CSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
  # CSV="eclipse-che-preview-kubernetes.v7.16.2-0.nightly"
  # eclipse-che-preview-kubernetes.v7.16.1
  # PackageVersion=$(echo "${CSV}" | sed -e "s/${packageName}.v//")
  PackageVersion="7.16.2-0.nightly"
}

add_Che_Cluster() {
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/manifests/che-operator.clusterserviceversion.yaml")
  CR=$(echo "$CRs" | yq -r ".[0]")
  CR=$(echo "$CR" | jq '.spec.server.tlsSupport = false')

  if [ "${platform}" == "kubernetes" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi

  echo "$CR" | kubectl apply -n "${NAMESPACE}" -f -
}

function getCheClusterLogs() {
  mkdir -p /root/payload/report/che-logs
  cd /root/payload/report/che-logs
  for POD in $(kubectl get pods -o name -n ${NAMESPACE}); do
    for CONTAINER in $(kubectl get -n ${NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
      echo ""
      echo "<=========================Getting logs from $POD==================================>"
      echo ""
      kubectl logs ${POD} -c ${CONTAINER} -n ${NAMESPACE} |tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
    done
  done
  echo "======== kubectl get events ========"
  kubectl get events -n "${NAMESPACE}" | tee get_events.log
  echo "======== kubectl get all ========"
  kubectl get all | tee get_all.log
}

function getOlmPodLogs() {
  mkdir -p /root/payload/report/olm-logs
  cd /root/payload/report/olm-logs
  for POD in $(kubectl get pods -o name -n olm); do
    for CONTAINER in $(kubectl get -n olm ${POD} -o jsonpath="{.spec.containers[*].name}"); do
      echo ""
      echo "<=========================Getting logs from $POD==================================>"
      echo ""
      kubectl logs ${POD} -c ${CONTAINER} -n olm |tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
    done
  done
}

run_olm_functions() {
  installOPM
  loginToImageRegistry

  OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
  OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="quay.io/${QUAY_USERNAME}/che_operator_bundle:0.0.1"
  buildBundleImage "${OPM_BUNDLE_MANIFESTS_DIR}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  CATALOG_IMAGENAME="quay.io/${QUAY_USERNAME}/testing_catalog:0.0.1"
  buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  createNamespace
  forcePullingOlmImages "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  installOperatorMarketPlace
  installPackage
  add_Che_Cluster
  waitCheServerDeploy
  getOlmPodLogs
  getCheClusterLogs
}

init
# $3 -> namespace
source ${BASE_DIR}/check-yq.sh
source ${BASE_DIR}/olm.sh ${platform} ${PackageVersion} $3 ${Install_Type}
run_olm_functions
