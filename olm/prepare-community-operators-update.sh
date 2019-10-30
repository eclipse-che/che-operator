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

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
source ${BASE_DIR}/check-yq.sh

for platform in 'kubernetes' 'openshift'
do
  packageName="eclipse-che-preview-${platform}"
  echo
  echo "## Preparing the OperatorHub package to push to the 'community-operators' repository for platform '${platform}' from local package '${packageName}'"

  packageBaseFolderPath="${BASE_DIR}/${packageName}"
  cd "${packageBaseFolderPath}"

  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  sourcePackageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  communityOperatorsLocalGitFolder="${packageBaseFolderPath}/generated/community-operators"
  lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "stable") | .currentCSV' "${sourcePackageFilePath}" | sed -e "s/${packageName}.v//")

  echo "   - Cloning the 'community-operators' GitHub repository to temporary folder: ${communityOperatorsLocalGitFolder}"
  
  rm -Rf "${communityOperatorsLocalGitFolder}"
  mkdir -p "${communityOperatorsLocalGitFolder}"
  git clone https://github.com/che-incubator/community-operators.git "${communityOperatorsLocalGitFolder}" 2>&1 | sed -e 's/^/      /'
  cd "${communityOperatorsLocalGitFolder}"
  git remote add upstream https://github.com/operator-framework/community-operators.git
  git fetch upstream master:upstream/master
  
  branch="update-eclipse-che"
  if [ "${platform}" == "kubernetes" ]
  then
    branch="${branch}-upstream"
  fi
  branch="${branch}-operator-${lastPackagePreReleaseVersion}"
  echo
  echo "   - Creating branch '${branch}' in the local 'community-operators' repository: ${communityOperatorsLocalGitFolder}"
  git checkout -b "${branch}" upstream/master 2>&1 | sed -e 's/^/      /'
  cd "${packageBaseFolderPath}"

  platformSubFolder="community-operators"
  if [ "${platform}" == "kubernetes" ]
  then
    platformSubFolder="upstream-${platformSubFolder}"
  fi

  folderToUpdate="${communityOperatorsLocalGitFolder}/${platformSubFolder}/eclipse-che"
  destinationPackageFilePath="${folderToUpdate}/eclipse-che.package.yaml"

  lastPublishedPackageVersion=$(yq -r '.channels[] | select(.name == "stable") | .currentCSV' "${destinationPackageFilePath}" | sed -e "s/eclipse-che.v//")
  echo
  echo "   - Last package pre-release version of local package: ${lastPackagePreReleaseVersion}"
  echo "   - Last package release version of cloned 'community-operators' repository: ${lastPublishedPackageVersion}"
  if [ "${lastPackagePreReleaseVersion}" == "${lastPublishedPackageVersion}" ]
  then
    echo "#### ERROR ####"
    echo "Release ${lastPackagePreReleaseVersion} already exists in the '${platformSubFolder}/eclipse-che' package !"
    exit 1
  fi

  echo "     => will create release '${lastPackagePreReleaseVersion}' in the following package folder :'${folderToUpdate}'"

  mkdir -p "${folderToUpdate}/${lastPackagePreReleaseVersion}"
  sed \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "/^  version: ${lastPackagePreReleaseVersion}/i\ \ replaces: eclipse-che.v${lastPublishedPackageVersion}" \
  -e "s/${packageName}/eclipse-che/" \
  "${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml" \
  > "${folderToUpdate}/${lastPackagePreReleaseVersion}/eclipse-che.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml"

  echo
  echo "   - Updating the CRD file"
  cp "${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.crd.yaml" \
  "${folderToUpdate}/${lastPackagePreReleaseVersion}/checlusters.org.eclipse.che.crd.yaml"
  echo
  echo "   - Updating the 'stable' channel with new release in the package descriptor: ${destinationPackageFilePath}"
  sed -e "s/${lastPublishedPackageVersion}/${lastPackagePreReleaseVersion}/" "${destinationPackageFilePath}" > "${destinationPackageFilePath}.new"
  mv "${destinationPackageFilePath}.new" "${destinationPackageFilePath}"
  echo
  echo "   - Committing changes"
  cd "${communityOperatorsLocalGitFolder}"
  git add --all
  git commit -s -m "Update eclipse-che operator for ${platform} to release ${lastPackagePreReleaseVersion}"
  echo
  echo "   - Pushing branch ${branch} to the 'che-incubator/community-operators' GitHub repository"
  if [ -z "${GIT_USER}" ] || [ -z "${GIT_PASSWORD}" ]
  then
    echo
    echo "#### WARNING ####"
    echo "####"
    echo "#### You shoud define GIT_USER and GIT_PASSWORD environment variable"
    echo "#### to be able to push release branches to the 'che-incubator/community-operators' repository"
    echo "####"
    echo "#### As soon as you have set them, you can push by running the following command:"
    echo "####    cd \"${communityOperatorsLocalGitFolder}\" && git push \"https://\${GIT_USER}:\${GIT_PASSWORD}@github.com/che-incubator/community-operators.git\" \"${branch}\""
    echo "####"
    echo "#################"
  else
    git push "https://${GIT_USER}:${GIT_PASSWORD}@github.com/che-incubator/community-operators.git" "${branch}"
  fi
done
cd "${CURRENT_DIR}"
