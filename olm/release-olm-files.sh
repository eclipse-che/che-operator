#!/bin/bash
#
# Copyright (c) 2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

REGEX="^([0-9]+)\\.([0-9]+)\\.([0-9]+)(\\-[0-9a-z-]+(\\.[0-9a-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
source ${BASE_DIR}/check-yq.sh

if [[ "$1" =~ $REGEX ]]
then
  RELEASE="$1"
else
  echo "You should provide the new release as the first parameter"
  echo "and it should be semver-compatible with optional *lower-case* pre-release part"
  exit 1
fi

for platform in 'kubernetes' 'openshift'
do
  packageName="eclipse-che-preview-${platform}"
  echo
  echo "## Creating release '${RELEASE}' of the OperatorHub package '${packageName}' for platform '${platform}'"

  packageBaseFolderPath="${BASE_DIR}/${packageName}"
  cd "${packageBaseFolderPath}"

  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  lastPackageNightlyVersion=$(yq -r '.channels[] | select(.name == "nightly") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")
  lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "stable") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")
  echo "   - Last package nightly version: ${lastPackageNightlyVersion}"
  echo "   - Last package pre-release version: ${lastPackagePreReleaseVersion}"
  if [ "${lastPackagePreReleaseVersion}" == "${RELEASE}" ]
  then
    echo "Release ${RELEASE} already exists in the package !"
    echo "You should first remove it"
    exit 1
  fi

  echo "     => will create release '${RELEASE}' from nightly version '${lastPackageNightlyVersion}' that will replace previous release '${lastPackagePreReleaseVersion}'"

  mkdir -p "${packageFolderPath}/${RELEASE}"
  sed \
  -e 's/imagePullPolicy: *Always/imagePullPolicy: IfNotPresent/' \
  -e 's/"cheImageTag": *"nightly"/"cheImageTag": ""/' \
  -e 's|"identityProviderImage": *"eclipse/che-keycloak:nightly"|"identityProviderImage": ""|' \
  -e 's|"devfileRegistryImage": *"quay.io/eclipse/che-devfile-registry:nightly"|"devfileRegistryImage": ""|' \
  -e 's|"pluginRegistryImage": *"quay.io/eclipse/che-plugin-registry:nightly"|"pluginRegistryImage": ""|' \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "s/^  version: ${lastPackageNightlyVersion}/  version: ${RELEASE}/" \
  -e "/^  version: ${RELEASE}/i\ \ replaces: ${packageName}.v${lastPackagePreReleaseVersion}" \
  -e "s/:nightly/:${RELEASE}/" \
  -e "s/${lastPackageNightlyVersion}/${RELEASE}/" \
  -e "s/createdAt:.*$/createdAt: \"$(date -u +%FT%TZ)\"/" \
  "${packageFolderPath}/${lastPackageNightlyVersion}/${packageName}.v${lastPackageNightlyVersion}.clusterserviceversion.yaml" \
  > "${packageFolderPath}/${RELEASE}/${packageName}.v${RELEASE}.clusterserviceversion.yaml"

  echo "   - Copying the CRD file"
  cp "${packageFolderPath}/${lastPackageNightlyVersion}/${packageName}.crd.yaml" \
  "${packageFolderPath}/${RELEASE}/${packageName}.crd.yaml"
  echo "   - Updating the 'stable' channel with new release in the package descriptor: ${packageFilePath}"
  sed -e "s/${lastPackagePreReleaseVersion}/${RELEASE}/" "${packageFilePath}" > "${packageFilePath}.new"
  mv "${packageFilePath}.new" "${packageFilePath}"

  diff -u "${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml" \
  "${packageFolderPath}/${RELEASE}/${packageName}.v${RELEASE}.clusterserviceversion.yaml" \
  > "${packageFolderPath}/${RELEASE}/${packageName}.v${RELEASE}.clusterserviceversion.yaml.diff" || true

  diff -u "${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.crd.yaml" \
  "${packageFolderPath}/${RELEASE}/${packageName}.crd.yaml" \
  > "${packageFolderPath}/${RELEASE}/${packageName}.crd.yaml.diff" || true
done
cd "${CURRENT_DIR}"
