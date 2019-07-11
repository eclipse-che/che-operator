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
  echo "## Preparing the OperatorHub package to push to the 'community-operators' repository for platform '${platform}' from local package '${packageName}'"

  packageBaseFolderPath=${BASE_DIR}/${packageName}
  cd ${packageBaseFolderPath}

  packageFolderPath=${packageBaseFolderPath}/deploy/olm-catalog/${packageName}
  communityOperatorsLocalGitFolder=${packageBaseFolderPath}/generated/community-operators

  echo "   - Cloning the 'community-operators' GitHub repository to temporary folder: ${communityOperatorsLocalGitFolder}"
  
  rm -Rf ${communityOperatorsLocalGitFolder}
  mkdir -p ${communityOperatorsLocalGitFolder}
  git clone https://github.com/operator-framework/community-operators.git ${communityOperatorsLocalGitFolder} 2>&1 | sed -e 's/^/      /'

  branch="update-eclipse-che"
  if [ "${platform}" == "kubernetes" ]
  then
    branch="${branch}-upstream"
  fi
  branch="${branch}-operator-$(date +%s)"
  echo "   - Creating branch '${branch}' in the local 'community-operators' repository: ${communityOperatorsLocalGitFolder}"
  cd ${communityOperatorsLocalGitFolder}
  git checkout -b ${branch} 2>&1 | sed -e 's/^/      /'
  cd ${packageBaseFolderPath}

  platformSubFolder="community-operators"
  if [ "${platform}" == "kubernetes" ]
  then
    platformSubFolder="upstream-${platformSubFolder}"
  fi

  folderToUpdate="${communityOperatorsLocalGitFolder}/${platformSubFolder}/eclipse-che"

  sourcePackageFilePath=${packageFolderPath}/${packageName}.package.yaml
  destinationPackageFilePath=${folderToUpdate}/eclipse-che.package.yaml

  lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "pre-releases") | .currentCSV' ${sourcePackageFilePath} | sed -e "s/${packageName}.v//")
  lastPublishedPackageVersion=$(yq -r '.channels[] | select(.name == "final") | .currentCSV' ${destinationPackageFilePath} | sed -e "s/eclipse-che.v//")
  echo "   - Last package pre-release version of local package: ${lastPackagePreReleaseVersion}"
  echo "   - Last package release version of cloned 'community-operators' repository: ${lastPackagePreReleaseVersion}"
  if [ ${lastPackagePreReleaseVersion} == ${lastPublishedPackageVersion} ]
  then
    echo "#### ERROR ####"
    echo "Release ${lastPackagePreReleaseVersion} already exists in the '${platformSubFolder}/eclipse-che' package !"
    exit 1
  fi

  echo "     => will create release '${lastPackagePreReleaseVersion}' in the following package folder :'${folderToUpdate}'"

  cat ${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml | sed \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "/^  version: ${lastPackagePreReleaseVersion}/i\ \ replaces: eclipse-che.v${lastPublishedPackageVersion}" \
  -e "s/${packageName}/eclipse-che/" \
  > ${folderToUpdate}/eclipse-che.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml

  echo "   - Copying the CRD file"
  cp ${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.crd.yaml \
  ${folderToUpdate}/eclipse-che.crd.yaml
  echo "   - Updating the 'final' channel with new release in the package descriptor: ${destinationPackageFilePath}"
  sed -e "s/${lastPublishedPackageVersion}/${lastPackagePreReleaseVersion}/" ${destinationPackageFilePath} > ${destinationPackageFilePath}.new
  mv ${destinationPackageFilePath}.new ${destinationPackageFilePath}
done
cd ${CURRENT_DIR}