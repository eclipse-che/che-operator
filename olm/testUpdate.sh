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
  if [ "${channel}" == "stable" ]; then
    packageName=eclipse-che-preview-${platform}
    platformPath=${OPERATOR_REPO}/olm/${packageName}
    packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
    packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

    LATEST_CSV_NAME=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
    lastPackageVersion=$(echo "${LATEST_CSV_NAME}" | sed -e "s/${packageName}.v//")
    PREVIOUS_CSV_NAME=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
    PACKAGE_VERSION=$(echo "${PREVIOUS_CSV_NAME}" | sed -e "s/${packageName}.v//")
    INSTALLATION_TYPE="Marketplace"
  else
    packageFolderPath="${OPERATOR_REPO}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
    PACKAGE_VERSION="nightly"
    export CATALOG_IMAGENAME="quay.io/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-catalog:0.0.1" #:preview
    INSTALLATION_TYPE="catalog"
  fi
}

run() {
  # $3 -> namespace
  source "${OPERATOR_REPO}/olm/olm.sh" "${platform}" "${PACKAGE_VERSION}" "${namespace}" "${INSTALLATION_TYPE}"

  createNamespace

  installOperatorMarketPlace

  if [ "${channel}" == "nightly" ]; then
    exposeCatalogSource
    getPreviousCSVInfo
    getLatestCSVInfo

    forcePullingOlmImages "${PREVIOUS_CSV_BUNDLE_IMAGE}"
    forcePullingOlmImages "${LATEST_CSV_BUNDLE_IMAGE}"
  fi

  subscribeToInstallation "${PREVIOUS_CSV_NAME}"
  echo -e "\u001b[32m Installation of the previous che-operator version: ${PREVIOUS_CSV_NAME} successfully completed \u001b[0m"
  installPackage
  applyCRCheCluster
  waitCheServerDeploy

  echo -e "\u001b[32m Installation of the latest che-operator version: ${LATEST_CSV_NAME} successfully completed \u001b[0m"
  installPackage
}

init
run
echo -e "\u001b[32m Done. \u001b[0m"
