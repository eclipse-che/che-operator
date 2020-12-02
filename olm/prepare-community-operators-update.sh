#!/bin/bash
#
# Copyright (c) 2019-2020 Red Hat, Inc.
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

FORCE="" # normally, don't allow pushing to an existing branch
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-f'|'--force') FORCE="-f";;
    '-h'|'--help') usage;;
  esac
  shift 1
done

usage ()
{
  echo "Usage: $0

Options:
    --force   |  if pull request branch already exists, force push new commits
"
}

for platform in 'kubernetes' 'openshift'
do
  packageName="eclipse-che-preview-${platform}"
  echo
  echo "## Prepare the OperatorHub package to push to the 'community-operators' repository for platform '${platform}' from local package '${packageName}'"

  packageBaseFolderPath="${BASE_DIR}/${packageName}"
  cd "${packageBaseFolderPath}"

  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  sourcePackageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  communityOperatorsLocalGitFolder="${packageBaseFolderPath}/generated/community-operators"
  lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "stable") | .currentCSV' "${sourcePackageFilePath}" | sed -e "s/${packageName}.v//")

  echo "   - Clone the 'community-operators' GitHub repository to temporary folder: ${communityOperatorsLocalGitFolder}"

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
  echo "   - Create branch '${branch}' in the local 'community-operators' repository: ${communityOperatorsLocalGitFolder}"
  git checkout upstream/master
  git checkout -b "${branch}" 2>&1 | sed -e 's/^/      /'
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
  if [[ "${lastPackagePreReleaseVersion}" == "${lastPublishedPackageVersion}" ]] && [[ "${FORCE}" == "" ]]; then
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
  echo "   - Update the CRD file"
  cp "${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.crd.yaml" \
  "${folderToUpdate}/${lastPackagePreReleaseVersion}/checlusters.org.eclipse.che.crd.yaml"
  echo
  echo "   - Update 'stable' channel with new release in the package descriptor: ${destinationPackageFilePath}"
  sed -e "s/${lastPublishedPackageVersion}/${lastPackagePreReleaseVersion}/" "${destinationPackageFilePath}" > "${destinationPackageFilePath}.new"
  mv "${destinationPackageFilePath}.new" "${destinationPackageFilePath}"
  echo

  echo "   - Generate ci.yaml file"
  echo "---
# Use \`replaces-mode\` or \`semver-mode\`. Once you switch to \`semver-mode\`, there is no easy way back.
updateGraph: replaces-mode" > ${folderToUpdate}/ci.yaml

  echo "   - Commit changes"
  cd "${communityOperatorsLocalGitFolder}"
  git add --all
  git commit -s -m "Update eclipse-che operator for ${platform} to release ${lastPackagePreReleaseVersion}"
  echo
  echo "   - Push branch ${branch} to the 'che-incubator/community-operators' GitHub repository"
  git push ${FORCE} "git@github.com:che-incubator/community-operators.git" "${branch}" 
done
cd "${CURRENT_DIR}"
