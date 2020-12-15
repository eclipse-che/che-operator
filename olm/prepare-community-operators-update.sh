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
source "${BASE_DIR}/check-yq.sh"

base_branch="master"
GITHUB_USER="che-bot"
fork_org="che-incubator"

FORCE="" # normally, don't allow pushing to an existing branch
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-u'|'--user') GITHUB_USER="$2"; shift 1;;
    '-t'|'--token') GITHUB_TOKEN="$2"; shift 1;;
    '-f'|'--force') FORCE="-f";;
    '-h'|'--help') usage;;
  esac
  shift 1
done
if [[ ! ${GITHUB_TOKEN} ]]; then 
  echo "Error: Must export GITHUB_TOKEN=[your token here] in order to generate pull request!"
  exit 1
fi

GIT_REMOTE_FORK="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${fork_org}/community-operators.git"
GIT_REMOTE_FORK_CLEAN="https://github.com/${fork_org}/community-operators.git"

usage ()
{
  echo "Usage: $0

Options:
    --force               |  if pull request branch already exists, force push new commits
    --user che-bot        |  specify which user to use for pull/push
    --token GITHUB_TOKEN  |  specify a token to use for pull/push, if not using 'export GITHUB_TOKEN=...'
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
  git clone "${GIT_REMOTE_FORK}" "${communityOperatorsLocalGitFolder}" 2>&1 | sed -e 's/^/      /'
  cd "${communityOperatorsLocalGitFolder}"
  git remote add upstream https://github.com/operator-framework/community-operators.git
  git fetch upstream ${base_branch}:upstream/${base_branch}

  branch="update-eclipse-che"
  if [ "${platform}" == "kubernetes" ]
  then
    branch="${branch}-upstream"
  fi
  branch="${branch}-operator-${lastPackagePreReleaseVersion}"
  echo
  echo "   - Create branch '${branch}' in the local 'community-operators' repository: ${communityOperatorsLocalGitFolder}"
  git checkout upstream/${base_branch}
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
  echo "   - Push branch ${branch} to ${GIT_REMOTE_FORK_CLEAN}"
  git push ${FORCE} origin "${branch}"

  echo
  template_file="https://raw.githubusercontent.com/operator-framework/community-operators/${base_branch}/docs/pull_request_template.md"
  HUB=$(command -v hub 2>/dev/null)
  if [[ $HUB ]] && [[ -x $HUB ]]; then 
    echo "   - Use $HUB to generate PR from template: ${template_file}"
    PRbody=$(curl -sSLo - ${template_file} | \
    sed -r -n '/#+ Updates to existing Operators/,$p' | sed -r -e "s#\[\ \]#[x]#g")

    lastCommitComment="$(git log -1 --pretty=%B)"
  $HUB pull-request -o -f -m "${lastCommitComment}

${lastCommitComment}

${PRbody}" -b "operator-framework:${base_branch}" -h "${fork_org}:${branch}"
  else 
    echo "hub is not installed. Install it from https://hub.github.com/ or submit PR manually using PR template:
${template_file}

${GIT_REMOTE_FORK_CLEAN}/pull/new/${branch}
"
  fi

done
cd "${CURRENT_DIR}"

echo 
echo "Generated pull requests will be here:

https://github.com/operator-framework/community-operators/pulls?q=is%3Apr+%22Update+eclipse-che+operator+for%22+is%3Aopen
"