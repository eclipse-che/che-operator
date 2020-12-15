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

init() {
  RELEASE="$1"
  BRANCH=$(echo $RELEASE | sed 's/.$/x/')
  RELEASE_BRANCH="${RELEASE}-release"
  GIT_REMOTE_UPSTREAM="https://github.com/eclipse/che-operator.git"
  RUN_RELEASE=false
  PUSH_OLM_FILES=false
  PUSH_GIT_CHANGES=false
  CREATE_PULL_REQUESTS=false
  RELEASE_OLM_FILES=false
  UPDATE_NIGHTLY_OLM_FILES=false
  PREPARE_COMMUNITY_OPERATORS_UPDATE=false
  RELEASE_DIR=$(cd "$(dirname "$0")"; pwd)
  FORCE_UPDATE=""

  if [[ $# -lt 1 ]]; then usage; exit; fi

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--release') RUN_RELEASE=true; shift 0;;
      '--push-olm-files') PUSH_OLM_FILES=true; shift 0;;
      '--push-git-changes') PUSH_GIT_CHANGES=true; shift 0;;
      '--pull-requests') CREATE_PULL_REQUESTS=true; shift 0;;
      '--release-olm-files') RELEASE_OLM_FILES=true; shift 0;;
      '--update-nightly-olm-files') UPDATE_NIGHTLY_OLM_FILES=true; shift 0;;
      '--prepare-community-operators-update') PREPARE_COMMUNITY_OPERATORS_UPDATE=true; shift 0;;
      '--force') FORCE_UPDATE="--force"; shift 0;;
    '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  [ -z "$QUAY_ECLIPSE_CHE_USERNAME" ] && echo "[ERROR] QUAY_ECLIPSE_CHE_USERNAME is not set" && exit 1
  [ -z "$QUAY_ECLIPSE_CHE_PASSWORD" ] && echo "[ERROR] QUAY_ECLIPSE_CHE_PASSWORD is not set" && exit 1
  command -v operator-courier >/dev/null 2>&1 || { echo "[ERROR] operator-courier is not installed. Abort."; exit 1; }
  command -v operator-sdk >/dev/null 2>&1 || { echo "[ERROR] operator-sdk is not installed. Abort."; exit 1; }
  command -v skopeo >/dev/null 2>&1 || { echo "[ERROR] skopeo is not installed. Abort."; exit 1; }
  REQUIRED_OPERATOR_SDK=$(yq -r ".\"operator-sdk\"" "${RELEASE_DIR}/REQUIREMENTS")
  [[ $(operator-sdk version) =~ .*${REQUIRED_OPERATOR_SDK}.* ]] || { echo "[ERROR] operator-sdk ${REQUIRED_OPERATOR_SDK} is required. Abort."; exit 1; }
  emptyDirs=$(find $RELEASE_DIR/olm/eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift/* -maxdepth 0 -empty | wc -l)
  [[ $emptyDirs -ne 0 ]] && echo "[ERROR] Found empty directories into eclipse-che-preview-openshift" && exit 1 || true
  emptyDirs=$(find $RELEASE_DIR/olm/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes/* -maxdepth 0 -empty | wc -l)
  [[ $emptyDirs -ne 0 ]] && echo "[ERROR] Found empty directories into eclipse-che-preview-openshift" && exit 1 || true
}

usage () {
	echo "Usage:   $0 [RELEASE_VERSION] --push-olm-files --push-git-changes"
  echo -e "\t--push-olm-files: to push OLM files to quay.io. This flag should be omitted "
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
    echo "[INFO] $BRANCH does not exist. Will be created a new one from master."
    resetChanges master
    git push origin master:$BRANCH
  fi
  git checkout -B $RELEASE_BRANCH
}

getPropertyValue() {
  local file=$1
  local key=$2
  echo $(cat $file | grep -m1 "$key" | tr -d ' ' | tr -d '\t' | cut -d = -f2)
}

checkImageReferences() {
  local filename=$1

  if ! grep -q "value: ${RELEASE}" $filename; then
    echo "[ERROR] Unable to find Che version ${RELEASE} in the $filename"; exit 1
  fi

  if ! grep -q "image: quay.io/eclipse/che-operator:$RELEASE" $filename; then
    echo "[ERROR] Unable to find Che operator image with version ${RELEASE} in the $filename"; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-server:$RELEASE" $filename; then
    echo "[ERROR] Unable to find Che server image with version ${RELEASE} in the $filename"; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-plugin-registry:$RELEASE" $filename; then
    echo "[ERROR] Unable to find plugin registry image with version ${RELEASE} in the $filename"; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-devfile-registry:$RELEASE" $filename; then
    echo "[ERROR] Unable to find devfile registry image with version ${RELEASE} in the $filename"; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-keycloak:$RELEASE" $filename; then
    echo "[ERROR] Unable to find che-keycloak image with version ${RELEASE} in the $filename"; exit 1
  fi

  if ! grep -q "value: $RELATED_IMAGE_pvc_jobs" $filename; then
    echo "[ERROR] Unable to find ubi8_minimal image in the $filename"; exit 1
  fi

  wget https://raw.githubusercontent.com/eclipse/che/${RELEASE}/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties

  plugin_broker_meta_image=$(cat /tmp/che.properties | grep  che.workspace.plugin_broker.metadata.image | cut -d '=' -f2)
  if ! grep -q "value: $plugin_broker_meta_image" $filename; then
    echo "[ERROR] Unable to find plugin broker meta image '$plugin_broker_meta_image' in the $filename"; exit 1
  fi

  plugin_broker_artifacts_image=$(cat /tmp/che.properties | grep  che.workspace.plugin_broker.artifacts.image | cut -d '=' -f2)
  if ! grep -q "value: $plugin_broker_artifacts_image" $filename; then
    echo "[ERROR] Unable to find plugin broker artifacts image '$plugin_broker_artifacts_image' in the $filename"; exit 1
  fi

  jwt_proxy_image=$(cat /tmp/che.properties | grep  che.server.secure_exposer.jwtproxy.image | cut -d '=' -f2)
  if ! grep -q "value: $jwt_proxy_image" $filename; then
    echo "[ERROR] Unable to find jwt proxy image $jwt_proxy_image in the $filename"; exit 1
  fi
}

releaseOperatorCode() {
  echo "[INFO] releaseOperatorCode :: Release operator code"
  echo "[INFO] releaseOperatorCode :: Launch 'replace-images-tags.sh' script"
  . ${RELEASE_DIR}/replace-images-tags.sh $RELEASE $RELEASE

  local operatoryaml=$RELEASE_DIR/deploy/operator.yaml
  echo "[INFO] releaseOperatorCode :: Validate changes for $operatoryaml"
  checkImageReferences $operatoryaml

  local operatorlocalyaml=$RELEASE_DIR/deploy/operator-local.yaml
  echo "[INFO] releaseOperatorCode :: Validate changes for $operatorlocalyaml"
  checkImageReferences $operatorlocalyaml

  echo "[INFO] releaseOperatorCode :: Commit changes"
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Update defaults tags to "$RELEASE --signoff
  fi

  echo "[INFO] releaseOperatorCode :: Build operator image"
  docker build -t "quay.io/eclipse/che-operator:${RELEASE}" .

  echo "[INFO] releaseOperatorCode :: Push image to quay.io"
  docker login quay.io -u "${QUAY_ECLIPSE_CHE_USERNAME}" -p "${QUAY_ECLIPSE_CHE_PASSWORD}"
  docker push quay.io/eclipse/che-operator:$RELEASE
}

updateNightlyOlmFiles() {
  echo "[INFO] updateNightlyOlmFiles :: Update nighlty OLM files"
  echo "[INFO] updateNightlyOlmFiles :: Launch 'olm/update-nightly-bundle.sh' script"

  export BASE_DIR=${RELEASE_DIR}/olm
  . ${BASE_DIR}/update-nightly-bundle.sh nightly
  unset BASE_DIR

  echo "[INFO] updateNightlyOlmFiles :: Commit changes"
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Update nightly olm files" --signoff
  fi
}

releaseOlmFiles() {
  echo "[INFO] releaseOlmFiles :: Release OLM files"
  echo "[INFO] releaseOlmFiles :: Launch 'olm/release-olm-files.sh' script"
  cd $RELEASE_DIR/olm
  . release-olm-files.sh $RELEASE
  cd $RELEASE_DIR

  local openshift=$RELEASE_DIR/olm/eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift
  local kubernetes=$RELEASE_DIR/olm/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes

  echo "[INFO] releaseOlmFiles :: Validate changes"
  grep -q "currentCSV: eclipse-che-preview-openshift.v"$RELEASE $openshift/eclipse-che-preview-openshift.package.yaml
  grep -q "currentCSV: eclipse-che-preview-kubernetes.v"$RELEASE $kubernetes/eclipse-che-preview-kubernetes.package.yaml
  grep -q "version: "$RELEASE $openshift/$RELEASE/eclipse-che-preview-openshift.v$RELEASE.clusterserviceversion.yaml
  grep -q "version: "$RELEASE $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.v$RELEASE.clusterserviceversion.yaml
  test -f $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.crd.yaml
  test -f $openshift/$RELEASE/eclipse-che-preview-openshift.crd.yaml

  echo "[INFO] releaseOlmFiles :: Commit changes"
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Release OLM files to "$RELEASE --signoff
  fi
}

pushOlmFilesToQuayIo() {
  echo "[INFO] Push OLM files to quay.io"
  cd $RELEASE_DIR/olm
  . push-olm-files-to-quay.sh
  cd $RELEASE_DIR
}

pushGitChanges() {
  echo "[INFO] Push git changes into $RELEASE_BRANCH branch"
  git push origin $RELEASE_BRANCH ${FORCE_UPDATE}
  if [[ $FORCE_UPDATE == "--force" ]]; then # if forced update, delete existing tag so we can replace it
    if git rev-parse "$RELEASE" >/dev/null 2>&1; then # if tag exists
      git tag -d $RELEASE
      git push origin :$RELEASE
    fi
  fi
  git tag -a $RELEASE -m $RELEASE
  git push --tags origin 
}

createPRToXBranch() {
  echo "[INFO] createPRToXBranch :: Create pull request into ${BRANCH} branch"
  if [[ $FORCE_UPDATE == "--force" ]]; then set +e; fi  # don't fail if PR already exists (just force push commits into it)
  hub pull-request $FORCE_UPDATE --base ${BRANCH} --head ${RELEASE_BRANCH} -m "Release version ${RELEASE}"
  set -e
}

createPRToMasterBranch() {
  echo "[INFO] createPRToMasterBranch :: Create pull request into master branch to copy csv"
  resetChanges master
  local tmpBranch="copy-csv-to-master"
  git checkout -B $tmpBranch
  git diff refs/heads/${BRANCH}...refs/heads/${RELEASE_BRANCH} ':(exclude)deploy/operator-local.yaml' ':(exclude)deploy/operator.yaml' | git apply -3
  . ${RELEASE_DIR}/replace-images-tags.sh nightly master
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Copy "$RELEASE" csv to master" --signoff
  fi
  git push origin $tmpBranch -f
  if [[ $FORCE_UPDATE == "--force" ]]; then set +e; fi  # don't fail if PR already exists (just force push commits into it)
  hub pull-request $FORCE_UPDATE --base master --head ${tmpBranch} -m "Copy "$RELEASE" csv to master"
  set -e
}

prepareCommunityOperatorsUpdate() {
  export BASE_DIR=${RELEASE_DIR}/olm
  "${BASE_DIR}/prepare-community-operators-update.sh" $FORCE_UPDATE
  unset BASE_DIR
}
run() {
  checkoutToReleaseBranch
  releaseOperatorCode
  if [[ $UPDATE_NIGHTLY_OLM_FILES == "true" ]]; then
    updateNightlyOlmFiles
  fi
  if [[ $RELEASE_OLM_FILES == "true" ]]; then
    releaseOlmFiles
  fi
}

init "$@"
echo "[INFO] Release '$RELEASE' from branch '$BRANCH'"

if [[ $RUN_RELEASE == "true" ]]; then
  run "$@"
fi

if [[ $PUSH_OLM_FILES == "true" ]]; then
  pushOlmFilesToQuayIo
fi

if [[ $PUSH_GIT_CHANGES == "true" ]]; then
  pushGitChanges
fi

if [[ $CREATE_PULL_REQUESTS == "true" ]]; then
  createPRToXBranch
  createPRToMasterBranch
fi

if [[ $PREPARE_COMMUNITY_OPERATORS_UPDATE == "true" ]]; then
  if [[ $UPDATE_NIGHTLY_OLM_FILES == "true" ]]; then
    updateNightlyOlmFiles
  fi
  prepareCommunityOperatorsUpdate
fi
