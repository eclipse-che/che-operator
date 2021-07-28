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
SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
PLATFORMS="kubernetes,openshift"
STABLE_CHANNELS=("stable-all-namespaces" "stable")
source "${BASE_DIR}/check-yq.sh"

base_branch="main"
GITHUB_USER="che-bot"
fork_org="che-incubator"

FORCE="" # normally, don't allow pushing to an existing branch
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-u'|'--user') GITHUB_USER="$2"; shift 1;;
    '-t'|'--token') GITHUB_TOKEN="$2"; shift 1;;
    '-f'|'--force') FORCE="-f";;
    '-p'|'--platform') PLATFORMS="$2";shift 1;;
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

. ${BASE_DIR}/olm/olm.sh
installOPM

for platform in $(echo $PLATFORMS | tr "," " ")
do
  INDEX_IMAGE="quay.io/eclipse/eclipse-che-${platform}-opm-catalog:preview"
  packageName="eclipse-che-preview-${platform}"
  echo
  echo "## Prepare the OperatorHub package to push to the 'community-operators' repository for platform '${platform}' from local package '${packageName}'"

  manifestPackagesDir=$(mktemp -d -t che-${platform}-manifest-packages-XXX)
  echo "[INFO] Folder with manifest packages: ${manifestPackagesDir}"
  ${OPM_BINARY} index export --index="${INDEX_IMAGE}" --package="${packageName}" -c="docker" --download-folder "${manifestPackagesDir}"
  packageBaseFolderPath="${manifestPackagesDir}/${packageName}"
  cd "${packageBaseFolderPath}"

  sourcePackageFilePath="${packageBaseFolderPath}/package.yaml"
  communityOperatorsLocalGitFolder="${packageBaseFolderPath}/generated/community-operators"
  lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "stable") | .currentCSV' "${sourcePackageFilePath}" | sed -e "s/${packageName}.v//")

  echo "   - Clone the 'community-operators' GitHub repository to temporary folder: ${communityOperatorsLocalGitFolder}"

  if [ "${platform}" == "openshift" ]
  then
    GIT_REMOTE_FORK="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${fork_org}/community-operators-prod.git"
    GIT_REMOTE_FORK_CLEAN="https://github.com/${fork_org}/community-operators-prod.git"
  fi
  rm -Rf "${communityOperatorsLocalGitFolder}"
  mkdir -p "${communityOperatorsLocalGitFolder}"
  git clone "${GIT_REMOTE_FORK}" "${communityOperatorsLocalGitFolder}" 2>&1 | sed -e 's/^/      /'
  cd "${communityOperatorsLocalGitFolder}"
  git remote add upstream https://github.com/k8s-operatorhub/community-operators
  if [ "${platform}" == "openshift" ]
  then
    git remote remove upstream
    git remote add upstream https://github.com/redhat-openshift-ecosystem/community-operators-prod
  fi
  
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

  platformSubFolder="operators"
  folderToUpdate="${communityOperatorsLocalGitFolder}/${platformSubFolder}/eclipse-che"
  destinationPackageFilePath="${folderToUpdate}/eclipse-che.package.yaml"

  for channel in "${STABLE_CHANNELS[@]}"
  do
    if [[ $channel == "stable-all-namespaces" && $platform == "kubernetes" ]];then
      continue
    fi
    lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "'$channel'") | .currentCSV' "${sourcePackageFilePath}" | sed -e "s/${packageName}.v//")
    lastPublishedPackageVersion=$(yq -r '.channels[] | select(.name == "'$channel'") | .currentCSV' "${destinationPackageFilePath}" | sed -e "s/eclipse-che.v//") 
    if [[ $channel == "stable-all-namespaces" && -z $lastPublishedPackageVersion ]];then
      lastPublishedPackageVersion=$lastPackagePreReleaseVersion
    fi

    echo
    echo "   - Last package pre-release version of local package: ${lastPackagePreReleaseVersion}"
    echo "   - Last package release version of cloned 'community-operators' repository: ${lastPublishedPackageVersion}"
    if [[ "${lastPackagePreReleaseVersion}" == "${lastPublishedPackageVersion}" ]] && [[ "${FORCE}" == "" ]]; then
      echo "#### ERROR ####"
      echo "Release ${lastPackagePreReleaseVersion} already exists in the '${platformSubFolder}/eclipse-che' package !"
      exit 1
    fi
    echo $lastPackagePreReleaseVersion
    echo $platform
    echo "     => will create release '${lastPackagePreReleaseVersion}' in the following package folder :'${folderToUpdate}'"

    mkdir -p "${folderToUpdate}/${lastPackagePreReleaseVersion}"
    sed \
    -e "/^  replaces: ${packageName}.v.*/d" \
    -e "/^  version: ${lastPackagePreReleaseVersion}/i\ \ replaces: eclipse-che.v${lastPublishedPackageVersion}" \
    -e "s/${packageName}/eclipse-che/" \
    "${packageBaseFolderPath}/${lastPackagePreReleaseVersion}/che-operator.clusterserviceversion.yaml" \
    > "${folderToUpdate}/${lastPackagePreReleaseVersion}/eclipse-che.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml"

    echo
    echo "   - Update the CRD files"
    cp "${packageBaseFolderPath}/${lastPackagePreReleaseVersion}/org_v1_che_crd.yaml" \
    "${folderToUpdate}/${lastPackagePreReleaseVersion}/checlusters.org.eclipse.che.crd.yaml"
    cp "${packageBaseFolderPath}/${lastPackagePreReleaseVersion}/org.eclipse.che_chebackupserverconfigurations_crd.yaml" "${folderToUpdate}/${lastPackagePreReleaseVersion}/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
    cp "${packageBaseFolderPath}/${lastPackagePreReleaseVersion}/org.eclipse.che_checlusterbackups_crd.yaml" "${folderToUpdate}/${lastPackagePreReleaseVersion}/org.eclipse.che_checlusterbackups_crd.yaml"
    cp "${packageBaseFolderPath}/${lastPackagePreReleaseVersion}/org.eclipse.che_checlusterrestores_crd.yaml" "${folderToUpdate}/${lastPackagePreReleaseVersion}/org.eclipse.che_checlusterrestores_crd.yaml"
    echo
    echo "   - Update 'stable' channel with new release in the package descriptor: ${destinationPackageFilePath}"
    sed -e "s/${lastPublishedPackageVersion}/${lastPackagePreReleaseVersion}/" "${destinationPackageFilePath}" > "${destinationPackageFilePath}.new"
    echo

    # Append to community operators the stable channel csv version: https://github.com/operator-framework/community-operators/blob/master/community-operators/eclipse-che/eclipse-che.package.yaml
    if [[ $channel == "stable" ]]; then
      mv "${destinationPackageFilePath}.new" "${destinationPackageFilePath}"
    fi

    # Append to community operators the stable-all-namespaces channel csv version: https://github.com/operator-framework/community-operators/blob/master/community-operators/eclipse-che/eclipse-che.package.yaml
    if [[ $channel == "stable-all-namespaces" ]]; then
      yq -riY ".channels[1] = { \"currentCSV\": \"eclipse-che.v${lastPackagePreReleaseVersion}\", \"name\": \"$channel\"}" $destinationPackageFilePath
    fi
  done
  # Make by default stable channel in the community operators eclipse-che.package.yaml
  yq -Yi '.defaultChannel |= "stable"' ${destinationPackageFilePath}

  # NOTE: if you update this file, you need to submit a PR against these two files:
  # https://github.com/redhat-openshift-ecosystem/community-operators-prod/blob/main/operators/eclipse-che/ci.yaml
  # https://github.com/k8s-operatorhub/community-operators/blob/main/operators/eclipse-che/ci.yaml
  echo "   - Replace ci.yaml file"
  cp ${BASE_DIR}/ci.yaml ${folderToUpdate}/ci.yaml

  echo "   - Commit changes"
  cd "${communityOperatorsLocalGitFolder}"
  git add --all
  git commit -s -m "Update eclipse-che operator for ${platform} to release ${lastPackagePreReleaseVersion}"
  echo
  echo "   - Push branch ${branch} to ${GIT_REMOTE_FORK_CLEAN}"
  git push ${FORCE} origin "${branch}"

  echo
  template_file="https://raw.githubusercontent.com/k8s-operatorhub/community-operators/${base_branch}/docs/pull_request_template.md"
  if [ "${platform}" == "openshift" ]
  then
    template_file="https://raw.githubusercontent.com/redhat-openshift-ecosystem/community-operators-prod/${base_branch}/docs/pull_request_template.md"
  fi
  HUB=$(command -v hub 2>/dev/null)

  upstream_org="k8s-operatorhub"
  if [ "${platform}" == "openshift" ]
  then
    upstream_org="redhat-openshift-ecosystem"
  fi
  if [[ $HUB ]] && [[ -x $HUB ]]; then
    echo "   - Use $HUB to generate PR from template: ${template_file}"
    PRbody=$(curl -sSLo - ${template_file} | \
    sed -r -n '/#+ Updates to existing Operators/,$p' | sed -r -e "s#\[\ \]#[x]#g")

    lastCommitComment="$(git log -1 --pretty=%B)"
  $HUB pull-request -f -m "${lastCommitComment}

${PRbody}" -b "${upstream_org}:${base_branch}" -h "${fork_org}:${branch}"
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

https://github.com/k8s-operatorhub/community-operators/pulls/che-incubator-bot
https://github.com/redhat-openshift-ecosystem/community-operators-prod/pulls/che-incubator-bot
"
