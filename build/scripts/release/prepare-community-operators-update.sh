#!/bin/bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
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

STABLE_CHANNELS=("stable")
OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/minikube-tests/common.sh"

base_branch="main"
GITHUB_USER="che-bot"
fork_org="che-incubator"

FORCE="" # normally, don't allow pushing to an existing branch
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-u'|'--user') GITHUB_USER="$2"; shift 1;;
    '-t'|'--token') GITHUB_TOKEN="$2"; shift 1;;
    '-f'|'--force') FORCE="-f"; shift 0;;
    '-h'|'--help') usage;;
  esac
  shift 1
done
if [[ ! ${GITHUB_TOKEN} ]]; then
  echo "Error: Must export GITHUB_TOKEN=[your token here] in order to generate pull request!"
  exit 1
fi

usage ()
{
  echo "Usage: $0

Options:
    --force               |  if pull request branch already exists, force push new commits
    --user che-bot        |  specify which user to use for pull/push
    --token GITHUB_TOKEN  |  specify a token to use for pull/push, if not using 'export GITHUB_TOKEN=...'
"
}

getLatestStableVersions

packageName="eclipse-che"
echo
echo "## Prepare the OperatorHub package to push to the 'community-operators-prod' repository from local package '${packageName}'"
manifestPackagesDir=$(mktemp -d -t che-openshift-manifest-packages-XXX)
echo "[INFO] Folder with manifest packages: ${manifestPackagesDir}"
packageBaseFolderPath="${manifestPackagesDir}/${packageName}"
sourcePackageFilePath="${packageBaseFolderPath}/package.yaml"
communityOperatorsLocalGitFolder="${packageBaseFolderPath}/generated/community-operators-prod"

echo "   - Clone the 'community-operators-prod' GitHub repository to temporary folder: ${communityOperatorsLocalGitFolder}"
GIT_REMOTE_FORK="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${fork_org}/community-operators-prod.git"
GIT_REMOTE_FORK_CLEAN="https://github.com/${fork_org}/community-operators-prod.git"
rm -Rf "${communityOperatorsLocalGitFolder}"
mkdir -p "${communityOperatorsLocalGitFolder}"
git clone "${GIT_REMOTE_FORK}" "${communityOperatorsLocalGitFolder}" 2>&1 | sed -e 's/^/      /'
cd "${communityOperatorsLocalGitFolder}"
git remote add upstream https://github.com/redhat-openshift-ecosystem/community-operators-prod

git fetch upstream ${base_branch}:upstream/${base_branch}

branch="update-eclipse-che"
branch="${branch}-operator-${LAST_PACKAGE_VERSION}"
echo
echo "   - Create branch '${branch}' in the local 'community-operators-prod' repository: ${communityOperatorsLocalGitFolder}"
git checkout upstream/${base_branch}
git checkout -b "${branch}" 2>&1 | sed -e 's/^/      /'

subFolder="operators"
folderToUpdate="${communityOperatorsLocalGitFolder}/${subFolder}/eclipse-che"
destinationPackageFilePath="${folderToUpdate}/eclipse-che.package.yaml"

for channel in "${STABLE_CHANNELS[@]}"
do
  getLatestStableVersions

  echo
  echo "   - Last package pre-release version of local package: ${LAST_PACKAGE_VERSION}"
  echo "   - Last package release version of cloned 'community-operators-prod' repository: ${PREVIOUS_PACKAGE_VERSION}"
  if [[ "${LAST_PACKAGE_VERSION}" == "${PREVIOUS_PACKAGE_VERSION}" ]] && [[ "${FORCE}" == "" ]]; then
    echo "#### ERROR ####"
    echo "Release ${LAST_PACKAGE_VERSION} already exists in the '${subFolder}/eclipse-che' package !"
    exit 1
  fi
  echo "     => will create release '${LAST_PACKAGE_VERSION}' in the following package folder :'${folderToUpdate}'"

  mkdir -p "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests"
  mkdir -p "${folderToUpdate}/${LAST_PACKAGE_VERSION}/metadata"
  echo
  sed \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "s/${packageName}/eclipse-che/" \
  "${OPERATOR_REPO}/bundle/$channel/${packageName}/manifests/che-operator.clusterserviceversion.yaml" \
  > "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/eclipse-che.v${LAST_PACKAGE_VERSION}.clusterserviceversion.yaml"

  echo "   - Update the CRD files"
  cp "${OPERATOR_REPO}/bundle/$channel/${packageName}/manifests/org.eclipse.che_checlusters.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/org.eclipse.che_checlusters.yaml"
  echo

  cp ${OPERATOR_REPO}/bundle/$channel/${packageName}/metadata/* "${folderToUpdate}/${LAST_PACKAGE_VERSION}/metadata"
  sed \
    -e '/operators.operatorframework.io.test.config.v1/d' \
    -e '/operators.operatorframework.io.test.mediatype.v1: scorecard+v1/d' \
    -i "${folderToUpdate}/${LAST_PACKAGE_VERSION}/metadata/annotations.yaml"

  cp "${OPERATOR_REPO}/bundle/$channel/${packageName}/manifests/eclipse-che-edit_rbac.authorization.k8s.io_v1_clusterrole.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/eclipse-che-edit_rbac.authorization.k8s.io_v1_clusterrole.yaml"
  cp "${OPERATOR_REPO}/bundle/$channel/${packageName}/manifests/eclipse-che-view_rbac.authorization.k8s.io_v1_clusterrole.yaml" "${folderToUpdate}/${LAST_PACKAGE_VERSION}/manifests/eclipse-che-view_rbac.authorization.k8s.io_v1_clusterrole.yaml"

  echo "   - Replace ci.yaml file"
  cp ${OPERATOR_REPO}/build/scripts/release/ci.yaml ${folderToUpdate}/ci.yaml

  echo "   - Commit changes"
  cd "${communityOperatorsLocalGitFolder}"
  git add --all
  commitMsg="Update eclipse-che operator to release ${LAST_PACKAGE_VERSION}"
  git commit -s -m "${commitMsg}"
  echo
  echo "   - Push branch ${branch} to ${GIT_REMOTE_FORK_CLEAN}"
  git push ${FORCE} origin "${branch}"

  echo
  template_file="https://raw.githubusercontent.com/redhat-openshift-ecosystem/community-operators-prod/${base_branch}/docs/pull_request_template.md"
  GH=$(command -v gh 2>/dev/null)
  upstream_org="redhat-openshift-ecosystem"
  if [[ $GH ]] && [[ -x $GH ]]; then
    echo "   - Use $GH to generate PR from template: ${template_file}"
    PRbody=$(curl -sSLo - ${template_file} | \
    sed -r -n '/#+ Updates to existing Operators/,$p' | sed -r -e "s#\[\ \]#[x]#g")

    $GH pr create --title "${commitMsg}" --body "${PRbody}" -H "${fork_org}:${branch}"
  else
    echo "gh is not installed. Install it from https://hub.github.com/ or submit PR manually using PR template:
${template_file}

${GIT_REMOTE_FORK_CLEAN}/pull/new/${branch}
"
  fi

done

echo
echo "Generated pull request:
https://github.com/redhat-openshift-ecosystem/community-operators-prod/pulls/che-incubator-bot
"
