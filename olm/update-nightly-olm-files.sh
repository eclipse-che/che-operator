#!/bin/bash
#
# Copyright (c) 2012-2018 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
for platform in 'kubernetes' 'openshift'
do
  packageName=eclipse-che-test-${platform}
  echo
  echo "## Updating OperatorHub package '${packageName}' for platform '${platform}'"
  packageBaseFolderPath=${BASE_DIR}/${packageName}
  cd "${packageBaseFolderPath}"
  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  lastPackageVersion=$(yq -r '.channels[] | select(.name == "nightly") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")
  echo "   - Last package version: ${lastPackageVersion}"
  newNightlyPackageVersion="0.0.0-nightly.$(date +%s)"
  echo "     => will create a new version: ${newNightlyPackageVersion}"
  ./build-roles.sh
  for role in "$(pwd)"/generated/roles/*.yaml
  do
    echo "   - Updating new package version with roles defined in: ${role}"
    cp "$role" generated/current-role.yaml
    operator-sdk olm-catalog gen-csv --csv-version "${newNightlyPackageVersion}" --from-version="${lastPackageVersion}" 2>&1 | sed -e 's/^/      /'
  done
  echo "   - Copying the CRD file"
  cp "${packageFolderPath}/${lastPackageVersion}/eclipse-che-test-${platform}.crd.yaml" "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-test-${platform}.crd.yaml"
  echo "   - Updating the 'nightly' channel with new version in the package descriptor: ${packageFilePath}"
  echo "     (the previous one is saved with the .old suffix)"
  sed -e "s/${lastPackageVersion}/${newNightlyPackageVersion}/" "${packageFilePath}" > "${packageFilePath}.new"
  mv "${packageFilePath}" "${packageFilePath}.old"
  mv "${packageFilePath}.new" "${packageFilePath}"
done
cd "${CURRENT_DIR}"