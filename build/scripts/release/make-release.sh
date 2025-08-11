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

set -e

init() {
  RELEASE="$1"
  DRY_RUN=false
  BRANCH=$(echo $RELEASE | sed 's/.$/x/')
  RELEASE_BRANCH="${RELEASE}-release"
  GIT_REMOTE_UPSTREAM="https://github.com/eclipse-che/che-operator.git"
  RUN_RELEASE=false
  PUSH_OLM_BUNDLES=false
  PUSH_GIT_CHANGES=false
  CREATE_PULL_REQUESTS=false
  RELEASE_OLM_FILES=false
  CHECK_RESOURCES=false
  PREPARE_COMMUNITY_OPERATORS_UPDATE=false
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
  FORCE_UPDATE=""
  BUILDX_PLATFORMS="linux/amd64,linux/arm64"
  CSV_STABLE_PATH=$(make csv-path CHANNEL=stable)
  MANAGER_YAML=${OPERATOR_REPO}/config/manager/manager.yaml
  CHE_OPERATOR_IMAGE=$(yq -r '.spec.template.spec.containers[0].image' "${MANAGER_YAML}" | sed -e "s/\(.*:\).*/\1${RELEASE}/")
  CHE_SERVER_IMAGE=$(yq -r '.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_che_server") | .value' "${MANAGER_YAML}" | sed -e "s/\(.*:\).*/\1${RELEASE}/")
  CHE_DASHBOARD_IMAGE=$(yq -r '.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_dashboard") | .value' "${MANAGER_YAML}" | sed -e "s/\(.*:\).*/\1${RELEASE}/")
  CHE_PLUGIN_REGISTRY_IMAGE=$(yq -r '.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_plugin_registry") | .value' "${MANAGER_YAML}" | sed -e "s/\(.*:\).*/\1${RELEASE}/")
  CHE_GATEWAY_IMAGE=$(yq -r '.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_single_host_gateway_config_sidecar") | .value' "${MANAGER_YAML}" | sed -e "s/\(.*:\).*/\1${RELEASE}/")

  if [[ $# -lt 1 ]]; then usage; exit; fi

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--release') RUN_RELEASE=true; shift 0;;
      '--dry-run') DRY_RUN=true; shift 0;;
      '--push-olm-bundles') PUSH_OLM_BUNDLES=true; shift 0;;
      '--push-git-changes') PUSH_GIT_CHANGES=true; shift 0;;
      '--pull-requests') CREATE_PULL_REQUESTS=true; shift 0;;
      '--release-olm-files') RELEASE_OLM_FILES=true; shift 0;;
      '--check-resources') CHECK_RESOURCES=true; shift 0;;
      '--prepare-community-operators-update') PREPARE_COMMUNITY_OPERATORS_UPDATE=true; shift 0;;
      '--force') FORCE_UPDATE="--force"; shift 0;;
    '--help'|'-h') usage; exit;;
    esac
    shift 1
  done
  [ -z "$QUAY_ECLIPSE_CHE_USERNAME" ] && echo "[ERROR] QUAY_ECLIPSE_CHE_USERNAME is not set" && exit 1
  [ -z "$QUAY_ECLIPSE_CHE_PASSWORD" ] && echo "[ERROR] QUAY_ECLIPSE_CHE_PASSWORD is not set" && exit 1
  command -v skopeo >/dev/null 2>&1 || { echo "[ERROR] skopeo is not installed. Abort."; exit 1; }
}

usage () {
  echo "Usage:   $0 [RELEASE_VERSION] --push-olm-bundles --push-git-changes"
  echo -e "\t--push-olm-bundles: to push OLM bundle images to quay.io and update catalog image. This flag should be omitted "
  echo -e "\t\tif already a greater version released. For instance, we are releasing 7.9.3 version but"
  echo -e "\t\t7.10.0 already exists. Otherwise it breaks the linear update path of the stable channel."
  echo -e "\t--push-git-changes: to create release branch and push changes into."
}

resetChanges() {
  echo "[INFO] Reset changes in $1 branch"
  git reset --hard
  git checkout $1
  git fetch ${GIT_REMOTE_UPSTREAM} --prune
  git pull ${GIT_REMOTE_UPSTREAM} $1
}

checkoutToReleaseBranch() {
  echo "[INFO] Check out to $BRANCH branch."
  local branchExist=$(git ls-remote -q --heads | grep $BRANCH | wc -l)
  if [[ $branchExist == 1 ]]; then
    echo "[INFO] $BRANCH exists."
    resetChanges $BRANCH
  else
    echo "[INFO] $BRANCH does not exist. Will be created a new one from main."
    resetChanges main
    git push origin main:$BRANCH
  fi
  git checkout -B $RELEASE_BRANCH
}

releaseManagerYaml() {
  echo "[INFO] releaseManagerYaml :: Update manager.yaml"

  yq -riY '(.spec.template.spec.containers[0].env[]  | select(.name == "CHE_VERSION") | .value ) = "'${RELEASE}'"' "${MANAGER_YAML}"
  yq -riY '(.spec.template.spec.containers[0].image) = "'${CHE_OPERATOR_IMAGE}'"' "${MANAGER_YAML}"
  yq -riY '(.spec.template.spec.containers[0].imagePullPolicy) = "IfNotPresent"' "${MANAGER_YAML}"
  yq -riY '(.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_che_server") | .value) = "'${CHE_SERVER_IMAGE}'"' "${MANAGER_YAML}"
  yq -riY '(.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_dashboard") | .value) = "'${CHE_DASHBOARD_IMAGE}'"' "${MANAGER_YAML}"
  yq -riY '(.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_plugin_registry") | .value) = "'${CHE_PLUGIN_REGISTRY_IMAGE}'"' "${MANAGER_YAML}"
  yq -riY '(.spec.template.spec.containers[0].env[] | select(.name=="RELATED_IMAGE_single_host_gateway_config_sidecar") | .value) = "'${CHE_GATEWAY_IMAGE}'"' "${MANAGER_YAML}"

  echo "[INFO] releaseManagerYaml :: Update editors definitions images"
  . "${OPERATOR_REPO}/build/scripts/release/editors-definitions.sh" update-manager-yaml \
      --yaml-path "${OPERATOR_REPO}/config/manager/manager.yaml"

  echo "[INFO] releaseManagerYaml :: Update samples images"
  . "${OPERATOR_REPO}/build/scripts/release/samples.sh" update-manager-yaml \
      --yaml-path "${OPERATOR_REPO}/config/manager/manager.yaml" \
      --index-json-url "https://raw.githubusercontent.com/eclipse-che/che-dashboard/${RELEASE}/packages/devfile-registry/air-gap/index.json"

  echo "[INFO] releaseManagerYaml :: Ensure license header"
  make license "${MANAGER_YAML}"

  echo "[INFO] releaseManagerYaml :: Commit changes"
  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Update manager.yaml to $RELEASE" --signoff
  fi
}

buildOperatorImage() {
  echo "[INFO] releaseOperatorCode :: Login to quay.io..."
  docker login quay.io -u "${QUAY_ECLIPSE_CHE_USERNAME}" -p "${QUAY_ECLIPSE_CHE_PASSWORD}"

  echo "[INFO] releaseOperatorCode :: Build operator image in platforms: $BUILDX_PLATFORMS"
  docker buildx build --platform "$BUILDX_PLATFORMS" --push -t "${CHE_OPERATOR_IMAGE}" .
}

updateVersionFile() {
  echo "[INFO] updating version.go file"
  # change version/version.go file
  sed -i version/version.go -r -e 's#(Version = ")([0-9.]+)(")#\1'"${RELEASE}"'\3#g'
  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Update version.go to $RELEASE" --signoff
  fi
}

releaseHelmPackage() {
  echo "[INFO] releaseHelmPackage :: release Helm package"
  yq -rYi ".version=\"${RELEASE}\"" "${OPERATOR_REPO}/helmcharts/stable/Chart.yaml"
  make update-helmcharts CHANNEL=stable
  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Update Helm Charts to $RELEASE" --signoff
  fi
}

releaseDeploymentFiles() {
  echo "[INFO] releaseDeploymentFiles :: Release Kubernetes resources"
  make gen-deployment

  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Update Kubernetes resources to $RELEASE" --signoff
  fi
}

releaseEditorsDefinitions() {
  echo "[INFO] releaseEditorsDefinitions :: Releasing editor definitions"
  . "${OPERATOR_REPO}/build/scripts/release/editors-definitions.sh" release --version "${RELEASE}"

  echo "[INFO] releaseEditorsDefinitions :: Ensure license header"
  make license editors-definitions

  echo "[INFO] releaseEditorsDefinitions :: Commit changes"
  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Release editors definitions to $RELEASE" --signoff
  fi
}

releaseOlmFiles() {
  echo "[INFO] releaseOlmFiles :: Make new bundle"
  make bundle CHANNEL=stable INCREMENT_BUNDLE_VERSION=false

  echo "[INFO] releaseOlmFiles :: Update che-operator.clusterserviceversion.yaml"
  yq -riY '(.spec.install.spec.deployments[].spec.template.spec.containers[0].image) = "'${CHE_OPERATOR_IMAGE}'"' "${CSV_STABLE_PATH}"
  yq -riY '(.metadata.annotations.containerImage) = "'${CHE_OPERATOR_IMAGE}'"' "${CSV_STABLE_PATH}"
  yq -riY '(.metadata.annotations.createdAt) = "'$(date -u +%FT%TZ)'"' "${CSV_STABLE_PATH}"
  yq -riY '(.spec.version) = "'${RELEASE}'"' "${CSV_STABLE_PATH}"
  yq -riY '(.metadata.name) = "eclipse-che.v'${RELEASE}'"' "${CSV_STABLE_PATH}"

  echo "[INFO] releaseOlmFiles :: Ensure license header"
  make license "${CSV_STABLE_PATH}"

  echo "[INFO] releaseOlmFiles :: Commit changes"
  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: New OLM $RELEASE bundle" --signoff
  fi
}

addDigests() {
  echo "[INFO] addDigests :: Pin images to digests"
  . "${OPERATOR_REPO}/build/scripts/release/addDigests.sh" -t "${RELEASE}" -s "${CSV_STABLE_PATH}" -o "${MANAGER_YAML}"

  echo "[INFO] addDigests :: Ensure license header"
  make license "${CSV_STABLE_PATH}"
  make license "${MANAGER_YAML}"

  echo "[INFO] addDigests :: Commit changes"
  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Pin images to digests" --signoff
  fi
}

pushOlmBundlesToQuayIo() {
  echo "[INFO] releaseOperatorCode :: Login to quay.io..."
  docker login quay.io -u "${QUAY_ECLIPSE_CHE_USERNAME}" -p "${QUAY_ECLIPSE_CHE_PASSWORD}"

  echo "[INFO] Push OLM bundles to quay.io"
  . "${OPERATOR_REPO}/build/scripts/olm/release-catalog.sh" --force -c stable -i quay.io/eclipse/eclipse-che-olm-catalog:stable

  if git status --porcelain; then
    git add -A || true
    git commit -am "ci: Add new $RELEASE bundle to a catalog" --signoff
  fi
}

pushGitChanges() {
  echo "[INFO] Push git changes into $RELEASE_BRANCH branch"
  git push origin $RELEASE_BRANCH ${FORCE_UPDATE}
  if [[ $FORCE_UPDATE == "--force" ]]; then # if forced update, delete existing tag so we can replace it
    if [[ $(git tag -l $RELEASE) ]]; then # if tag exists in local repo
      echo "Remove existing local tag $RELEASE"
      git tag -d $RELEASE
    else
      echo "Local tag $RELEASE does not exist" # should never get here
    fi
    if [[ $(git ls-remote --tags $(git remote get-url origin) $RELEASE) ]]; then # if tag exists in remote repo
      echo "Remove existing remote tag $RELEASE"
      git push origin :$RELEASE
    else
      echo "Remote tag $RELEASE does not exist" # should never get here
    fi
  fi
  git tag -a $RELEASE -m $RELEASE
  git push --tags origin
}

createPRToXBranch() {
  echo "[INFO] createPRToXBranch :: Create pull request into ${BRANCH} branch"
  if [[ $FORCE_UPDATE == "--force" ]]; then set +e; fi  # don't fail if PR already exists (just force push commits into it)
  gh pr create -f -B ${BRANCH} -H ${RELEASE_BRANCH}
  set -e
}

createPRToMainBranch() {
  echo "[INFO] createPRToMainBranch :: Create pull request into main branch to copy csv"
  resetChanges main
  local tmpBranch="copy-csv-to-main"
  git checkout -B $tmpBranch
  git diff refs/heads/${BRANCH}...refs/heads/${RELEASE_BRANCH} ':(exclude)config/manager/manager.yaml' ':(exclude)deploy' ':(exclude)editors-definitions' ':(exclude)Dockerfile' | git apply -3
  if git status --porcelain; then
    git add -A || true # add new generated files
    git commit -am "ci: Copy "$RELEASE" csv to main" --signoff
  fi
  git push origin $tmpBranch -f
  if [[ $FORCE_UPDATE == "--force" ]]; then set +e; fi  # don't fail if PR already exists (just force push commits into it)
  gh pr create -f -B main -H ${tmpBranch}
  set -e
}

prepareCommunityOperatorsUpdate() {
  . "${OPERATOR_REPO}/build/scripts/release/prepare-community-operators-update.sh" $FORCE_UPDATE
}

run() {
  if [[ $CHECK_RESOURCES == "true" ]]; then
    echo "[INFO] Check if resources are up to date"
    . ${OPERATOR_REPO}/build/scripts/check-resources.sh
  fi

  checkoutToReleaseBranch
  updateVersionFile
  releaseEditorsDefinitions
  releaseManagerYaml
  if [[ $DRY_RUN == "false" ]]; then
    buildOperatorImage
  else
    echo "[INFO] Dry run is enabled. Skipping buildOperatorImage"
  fi
  if [[ $RELEASE_OLM_FILES == "true" ]]; then
    releaseOlmFiles
    addDigests
  fi
  # Must be done after launching `addDigest.sh`
  releaseDeploymentFiles
  releaseHelmPackage
}

init "$@"
echo "[INFO] Release '$RELEASE' from branch '$BRANCH'"

if [[ $RUN_RELEASE == "true" ]]; then
  run "$@"
fi

if [[ $PUSH_OLM_BUNDLES == "true" ]]; then
  if [[ $DRY_RUN == "false" ]]; then
    pushOlmBundlesToQuayIo
  else
    echo "[INFO] Dry run is enabled. Skipping pushOlmBundlesToQuayIo"
  fi
fi

if [[ $PUSH_GIT_CHANGES == "true" ]]; then
  pushGitChanges
fi

if [[ $CREATE_PULL_REQUESTS == "true" ]]; then
  createPRToXBranch
  createPRToMainBranch
fi

if [[ $PREPARE_COMMUNITY_OPERATORS_UPDATE == "true" ]]; then
  if [[ $DRY_RUN == "false" ]]; then
    prepareCommunityOperatorsUpdate
  else
    echo "[INFO] Dry run is enabled. Skipping prepareCommunityOperatorsUpdate"
  fi
fi
