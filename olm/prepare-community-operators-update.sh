#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

CURRENT_DIR=$(pwd)
SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
BASE_DIR=$(dirname "$(dirname "$SCRIPT")")
PLATFORMS="openshift"
STABLE_CHANNELS=("tech-preview-stable-all-namespaces" "stable")
source "${BASE_DIR}/olm/check-yq.sh"

base_branch="main"
GITHUB_USER="che-bot"
fork_org="che-incubator"

FORCE="" # normally, don't allow pushing to an existing branch
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-u'|'--user') GITHUB_USER="$2"; shift 1;;
    '-t'|'--token') GITHUB_TOKEN="$2"; shift 1;;
    '-f'|'--force') FORCE="-f"; shift 0;;
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

# $BASE_DIR is set to {OPERATOR_DIR}/olm
OPERATOR_REPO=$(dirname "$BASE_DIR")
source ${OPERATOR_REPO}/.github/bin/common.sh
getLatestsStableVersions

for platform in $(echo $PLATFORMS | tr "," " ")
do
  INDEX_IMAGE="quay.io/eclipse/eclipse-che-${platform}-opm-catalog:test"
  packageName="eclipse-che-preview-${platform}"
  echo
  echo "## Prepare the OperatorHub package to push to the 'community-operators' repository for platform '${platform}' from local package '${packageName}'"
  manifestPackagesDir=$(mktemp -d -t che-${platform}-manifest-packages-XXX)
  echo "[INFO] Folder with manifest packages: ${manifestPackagesDir}"
  packageBaseFolderPath="${manifestPackagesDir}/${packageName}"

  sourcePackageFilePath="${packageBaseFolderPath}/package.yaml"
  communityOperatorsLocalGitFolder="${packageBaseFolderPath}/generated/community-operators"

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
  branch="${branch}-operator-${LAST_PACKAGE_VERSION}"
  echo
  echo "   - Create branch '${branch}' in the local 'community-operators' repository: ${communityOperatorsLocalGitFolder}"
  git checkout upstream/${base_branch}
  git checkout -b "${branch}" 2>&1 | sed -e 's/^/      /'

  platformSubFolder="operators"
  folderToUpdate="${communityOperatorsLocalGitFolder}/${platformSubFolder}/eclipse-che"
  destinationPackageFilePath="${folderToUpdate}/eclipse-che.package.yaml"

  for channel in "${STABLE_CHANNELS[@]}"
  do
    getLatestsStableVersions

    if [[ $channel == "tech-preview-stable-all-namespaces" ]];then
      # Add suffix for stable-<all-namespaces> channel
      LAST_PACKAGE_VERSION="$LAST_PACKAGE_VERSION-all-namespaces"
      PREVIOUS_PACKAGE_VERSION="${PREVIOUS_PACKAGE_VERSION}-all-namespaces"
    fi

    echo
    echo "   - Last package pre-release version of local package: ${LAST_PACKAGE_VERSION}"
    echo "   - Last package release version of cloned 'community-operators' repository: ${PREVIOUS_PACKAGE_VERSION}"
    if [[ "${LAST_PACKAGE_VERSION}" == "${PREVIOUS_PACKAGE_VERSION}" ]] && [[ "${FORCE}" == "" ]]; then
      echo "#### ERROR ####"
      echo "Release ${LAST_PACKAGE_VERSION} already exists in the '${platformSubFolder}/eclipse-che' package !"
      exit 1
    fi
    echo "     => will create release '${LAST_PACKAGE_VERSION}' in the following package folder :'${folderToUpdate}'"

    mkdir -p "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests"
    mkdir -p "${folderToUpdate}/${LAST_PACKAGE_VERSION}/metadata"
    echo
    sed \
    -e "/^  replaces: ${packageName}.v.*/d" \
    -e "/^  version: ${LAST_PACKAGE_VERSION}/i\ \ replaces: eclipse-che.v${PREVIOUS_PACKAGE_VERSION}" \
    -e "s/${packageName}/eclipse-che/" \
    "${OPERATOR_REPO}/bundle/$channel/eclipse-che-preview-$platform/manifests/che-operator.clusterserviceversion.yaml" \
    > "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/eclipse-che.v${LAST_PACKAGE_VERSION}.clusterserviceversion.yaml"

    echo "   - Update the CRD files"
    cp "${OPERATOR_REPO}/bundle/$channel/eclipse-che-preview-$platform/manifests/org_v1_che_crd.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/checlusters.org.eclipse.che.crd.yaml"
    cp "${OPERATOR_REPO}/bundle/$channel/eclipse-che-preview-$platform/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
    cp "${OPERATOR_REPO}/bundle/$channel/eclipse-che-preview-$platform/manifests//org.eclipse.che_checlusterbackups_crd.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/org.eclipse.che_checlusterbackups_crd.yaml"
    cp "${OPERATOR_REPO}/bundle/$channel/eclipse-che-preview-$platform/manifests//org.eclipse.che_checlusterrestores_crd.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/org.eclipse.che_checlusterrestores_crd.yaml"
    echo

    cp ${OPERATOR_REPO}/bundle/$channel/eclipse-che-preview-$platform/metadata/* "${folderToUpdate}/${LAST_PACKAGE_VERSION}/metadata"
    sed \
      -e 's/operators.operatorframework.io.bundle.package.v1: eclipse-che-preview-'${platform}'/operators.operatorframework.io.bundle.package.v1: eclipse-che/' \
      -e '/operators.operatorframework.io.test.config.v1/d' \
      -e '/operators.operatorframework.io.test.mediatype.v1: scorecard+v1/d' \
      -i "${folderToUpdate}/${LAST_PACKAGE_VERSION}/metadata/annotations.yaml"
  done

  # NOTE: if you update this file, you need to submit a PR against these two files:
  # https://github.com/redhat-openshift-ecosystem/community-operators-prod/blob/main/operators/eclipse-che/ci.yaml
  # https://github.com/k8s-operatorhub/community-operators/blob/main/operators/eclipse-che/ci.yaml
  echo "   - Replace ci.yaml file"
  cp ${BASE_DIR}/ci.yaml ${folderToUpdate}/ci.yaml

  echo "   - Commit changes"
  cd "${communityOperatorsLocalGitFolder}"
  git add --all
  git commit -s -m "Update eclipse-che operator for ${platform} to release ${LAST_PACKAGE_VERSION}"
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

https://github.com/redhat-openshift-ecosystem/community-operators-prod/pulls/che-incubator-bot
"
