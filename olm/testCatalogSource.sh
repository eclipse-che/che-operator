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

  # Setting The catalog image and the image and tag; and install type
  Install_Type="LocalCatalog"
  CATALOG_IMAGENAME="testing_catalog:0.0.1"
  
  # GET the package version to apply
  packageName=eclipse-che-preview-${platform}
  packageFolderPath="${BASE_DIR}/eclipse-che-preview-${platform}/deploy/olm-catalog/${packageName}"
  packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  CSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
  PackageVersion=$(echo "${CSV}" | sed -e "s/${packageName}.v//")
}

docker_build() {
  docker build -t ${CATALOG_IMAGENAME} -f "${BASE_DIR}"/eclipse-che-preview-"${platform}"/Dockerfile \
    "${BASE_DIR}"/eclipse-che-preview-"${platform}"
}

add_Che_Cluster() {
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/${PackageVersion}/${packageName}.v${PackageVersion}.clusterserviceversion.yaml")
  CR=$(echo "$CRs" | yq -r ".[0]")
  CR=$(echo "$CR" | jq '.spec.server.tlsSupport = false')
  
  if [ "${platform}" == "kubernetes" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi

  echo "$CR" | kubectl apply -n "${namespace}" -f -
}

build_Catalog_Image() {
  if [ "${platform}" == "kubernetes" ]; then
    eval "$(minikube -p minikube docker-env)"
    docker_build
    minikube addons enable ingress
  else
    docker_build
    curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.14.1/install.sh | bash -s 0.14.1
    docker save ${CATALOG_IMAGENAME} > /tmp/catalog.tar
    eval "$(minishift docker-env)"
    docker load -i /tmp/catalog.tar && rm -rf /tmp/catalog.tar
  fi
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
  kubectl get events -n ${NAMESPACE}| tee get_events.log
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
  build_Catalog_Image
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
