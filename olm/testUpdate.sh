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

SCRIPT=$(readlink -f "$0")
SCRIPT_DIR=$(dirname "$SCRIPT")
BASE_DIR=$(dirname "$SCRIPT_DIR");

source ${BASE_DIR}/olm/check-yq.sh

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
platformPath=${BASE_DIR}/olm/${packageName}
packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

lastCSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")
previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
previousPackageVersion=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

# $3 -> namespace
source ${BASE_DIR}/olm/olm.sh ${platform} ${previousPackageVersion} $3

installOperatorMarketPlace
installPackage
applyCRCheCluster
waitCheServerDeploy

echo -e "\u001b[32m Installation of the previous che-operator version: ${previousCSV} succesfully completed \u001b[0m"

installPackage
