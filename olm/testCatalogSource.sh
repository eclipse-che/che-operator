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

BASE_DIR=$(cd "$(dirname "$0")" && pwd)

Install_Type="LocalCatalog"
QUAY_PROJECT=catalogsource
CATALOG_IMAGENAME="quay.io/${QUAY_USERNAME}/${QUAY_PROJECT}"

source ${BASE_DIR}/check-yq.sh

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

packageName=eclipse-che-preview-${platform}
packageFolderPath="${BASE_DIR}/eclipse-che-preview-${platform}/deploy/olm-catalog/${packageName}"
packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

CSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
PackageVersion=$(echo "${CSV}" | sed -e "s/${packageName}.v//")

# $3 -> namespace
source ${BASE_DIR}/olm.sh ${platform} ${PackageVersion} $3 ${Install_Type}
build_Catalog_Image
installOperatorMarketPlace
installPackage
applyCRCheCluster
waitCheServerDeploy