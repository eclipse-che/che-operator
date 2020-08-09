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

PLATFORM=$1
if [ "${PLATFORM}" == "" ]; then
  echo "Please specify PLATFORM ('openshift' or 'kubernetes') as the first argument."
  echo ""
  echo "testUpdate.sh <PLATFORM> [<channel>] [<namespace>]"
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
  Install_Type="catalog"
  packageFolderPath="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${PLATFORM}"
  PACKAGE_VERSION="7.16.2-0.nightly"
}

add_Che_Cluster() {
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/manifests/che-operator.clusterserviceversion.yaml")
  CR=$(echo "$CRs" | yq -r ".[0]")
  CR=$(echo "$CR" | jq '.spec.server.tlsSupport = false')

  if [ "${PLATFORM}" == "kubernetes" ]
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

run() {
  source "${BASE_DIR}/olm.sh" "${PLATFORM}" "${PACKAGE_VERSION}" "${NAMESPACE}" "${Install_Type}"
  installOPM
  loginToImageRegistry

  export CATALOG_IMAGENAME="quay.io/${QUAY_USERNAME}/eclipse-che-${PLATFORM}-opm-catalog:0.0.1"

  createNamespace

  installOperatorMarketPlace

  exposeCatalogSource
  getPreviousCSVInfo
  getLatestCSVInfo

  forcePullingOlmImages "${PREVIOUS_CSV_BUNDLE_IMAGE}"
  forcePullingOlmImages "${LATEST_CSV_BUNDLE_IMAGE}"

  subscribeToInstallation "${PREVIOUS_CSV_NAME}"
  installPackage
  add_Che_Cluster
  waitCheServerDeploy
  getOlmPodLogs
  getCheClusterLogs

  installPackage
}

init
run
