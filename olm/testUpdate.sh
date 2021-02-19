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

if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "$0")
  OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")");
fi

source ${OPERATOR_REPO}/olm/check-yq.sh

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

namespace=$3
if [ "${namespace}" == "" ]; then
  namespace="eclipse-che-preview-test"
fi

init() {
  IMAGE_REGISTRY_HOST=${IMAGE_REGISTRY_HOST:-quay.io}
  IMAGE_REGISTRY_USER_NAME=${IMAGE_REGISTRY_USER_NAME:-eclipse}
  export CATALOG_IMAGENAME="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-catalog:preview"

  source "${OPERATOR_REPO}/olm/olm.sh"
  initOLMScript "${platform}" "${namespace}"

  OPM_BUNDLE_DIR=$(getBundlePath "${channel}")
  CSV_FILE_PATH="${OPM_BUNDLE_DIR}/manifests/che-operator.clusterserviceversion.yaml"
}

run() {
  createNamespace

  installOperatorMarketPlace
  installCatalogSource "${CATALOG_IMAGENAME}"

  getBundleListFromCatalogSource
  getPreviousCSVInfo "${channel}"
  getLatestCSVInfo "${channel}"

  forcePullingOlmImages "${PREVIOUS_CSV_BUNDLE_IMAGE}"
  forcePullingOlmImages "${LATEST_CSV_BUNDLE_IMAGE}"

  subscribeToInstallation "${channel}" "${PREVIOUS_CSV_NAME}"
  installPackage
  echo -e "\u001b[32m Installation of the previous che-operator version: ${PREVIOUS_CSV_NAME} successfully completed \u001b[0m"
  applyCRCheCluster "${CSV_FILE_PATH}"
  waitCheServerDeploy

  installPackage
  echo -e "\u001b[32m Installation of the latest che-operator version: ${LATEST_CSV_NAME} successfully completed \u001b[0m"
}

init
run
echo -e "\u001b[32m Done. \u001b[0m"
