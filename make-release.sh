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
  GIT_REMOTE_UPSTREAM="https://github.com/eclipse-che/che-operator.git"
  RUN_RELEASE=false
  PUSH_OLM_BUNDLES=false
  PUSH_GIT_CHANGES=false
  CREATE_PULL_REQUESTS=false
  RELEASE_OLM_FILES=false
  CHECK_RESOURCES=false
  PREPARE_COMMUNITY_OPERATORS_UPDATE=false
  RELEASE_DIR=$(cd "$(dirname "$0")"; pwd)
  FORCE_UPDATE=""
  BUILDX_PLATFORMS="linux/amd64,linux/ppc64le"
  DEV_WORKSPACE_CONTROLLER_VERSION="main"
  DEV_WORKSPACE_CHE_OPERATOR_VERSION="main"

  if [[ $# -lt 1 ]]; then usage; exit; fi

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--release') RUN_RELEASE=true; shift 0;;
      '--push-olm-bundles') PUSH_OLM_BUNDLES=true; shift 0;;
      '--push-git-changes') PUSH_GIT_CHANGES=true; shift 0;;
      '--pull-requests') CREATE_PULL_REQUESTS=true; shift 0;;
      '--release-olm-files') RELEASE_OLM_FILES=true; shift 0;;
      '--check-resources') CHECK_RESOURCES=true; shift 0;;
      '--prepare-community-operators-update') PREPARE_COMMUNITY_OPERATORS_UPDATE=true; shift 0;;
      '--dev-workspace-controller-version') DEV_WORKSPACE_CONTROLLER_VERSION=$2; shift 1;;
      '--dev-workspace-che-operator-version') DEV_WORKSPACE_CHE_OPERATOR_VERSION=$2; shift 1;;
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
  command -v pysemver >/dev/null 2>&1 || { echo "[ERROR] pysemver is not installed. Abort."; exit 1; }
  REQUIRED_OPERATOR_SDK=$(yq -r ".\"operator-sdk\"" "${RELEASE_DIR}/REQUIREMENTS")
  [[ $(operator-sdk version) =~ .*${REQUIRED_OPERATOR_SDK}.* ]] || { echo "[ERROR] operator-sdk ${REQUIRED_OPERATOR_SDK} is required. Abort."; exit 1; }
}

usage () {
	echo "Usage:   $0 [RELEASE_VERSION] --push-olm-files --push-git-changes"
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

if ! grep -q "value: quay.io/eclipse/che-dashboard:$RELEASE" $filename; then
    echo "[ERROR] Unable to find dashboard image with version ${RELEASE} in the $filename"; exit 1
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
  echo "[INFO] releaseOperatorCode :: Replacing tags"
  replaceImagesTags

  local operatoryaml=$RELEASE_DIR/deploy/operator.yaml
  echo "[INFO] releaseOperatorCode :: Validate changes for $operatoryaml"
  checkImageReferences $operatoryaml

  echo "[INFO] releaseOperatorCode :: Commit changes"
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Update defaults tags to "$RELEASE --signoff
  fi
  echo "[INFO] releaseOperatorCode :: Login to quay.io..."
  docker login quay.io -u "${QUAY_ECLIPSE_CHE_USERNAME}" -p "${QUAY_ECLIPSE_CHE_PASSWORD}"

  echo "[INFO] releaseOperatorCode :: Build operator image in platforms: $BUILDX_PLATFORMS"
  docker buildx build --build-arg DEV_WORKSPACE_CONTROLLER_VERSION=${DEV_WORKSPACE_CONTROLLER_VERSION} --build-arg DEV_WORKSPACE_CHE_OPERATOR_VERSION=${DEV_WORKSPACE_CHE_OPERATOR_VERSION} --platform "$BUILDX_PLATFORMS" --push -t "quay.io/eclipse/che-operator:${RELEASE}" .
}

replaceImagesTags() {
  OPERATOR_YAML="${RELEASE_DIR}/deploy/operator.yaml"

  lastDefaultCheServerImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value" "${OPERATOR_YAML}")
  lastDefaultDashboardImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_dashboard\") | .value" "${OPERATOR_YAML}")
  lastDefaultDevWorkspaceControllerImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_devworkspace_controller\") | .value" "${OPERATOR_YAML}")
  lastDefaultDevWorkspaceCheOperatorImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_devworkspace_che_operator\") | .value" "${OPERATOR_YAML}")
  lastDefaultKeycloakImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value" "${OPERATOR_YAML}")
  lastDefaultPluginRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value" "${OPERATOR_YAML}")
  lastDefaultDevfileRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value" "${OPERATOR_YAML}")

  CHE_SERVER_IMAGE_REALEASE=$(replaceTag "${lastDefaultCheServerImage}" "${RELEASE}")
  DASHBOARD_IMAGE_REALEASE=$(replaceTag "${lastDefaultDashboardImage}" "${RELEASE}")
  DEVWORKSPACE_CONTROLLER_IMAGE_RELEASE=$(replaceTag "${lastDefaultDevWorkspaceControllerImage}" "${RELEASE}")
  DEVWORKSPACE_CHE_OPERATOR_IMAGE_RELEASE=$(replaceTag "${lastDefaultDevWorkspaceCheOperatorImage}" "${RELEASE}")
  KEYCLOAK_IMAGE_RELEASE=$(replaceTag "${lastDefaultKeycloakImage}" "${RELEASE}")
  PLUGIN_REGISTRY_IMAGE_RELEASE=$(replaceTag "${lastDefaultPluginRegistryImage}" "${RELEASE}")
  DEVFILE_REGISTRY_IMAGE_RELEASE=$(replaceTag "${lastDefaultDevfileRegistryImage}" "${RELEASE}")

  NEW_OPERATOR_YAML="${OPERATOR_YAML}.new"
  # copy licence header
  eval head -10 "${OPERATOR_YAML}" > ${NEW_OPERATOR_YAML}

  cat "${OPERATOR_YAML}" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\") | .image ) = \"quay.io/eclipse/che-operator:${RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"CHE_VERSION\") | .value ) = \"${RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value ) = \"${CHE_SERVER_IMAGE_REALEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_dashboard\") | .value ) = \"${DASHBOARD_IMAGE_REALEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_devworkspace_controller\") | .value ) = \"${DEVWORKSPACE_CONTROLLER_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_devworkspace_che_operator\") | .value ) = \"${DEVWORKSPACE_CHE_OPERATOR_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value ) = \"${KEYCLOAK_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value ) = \"${PLUGIN_REGISTRY_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value ) = \"${DEVFILE_REGISTRY_IMAGE_RELEASE}\"" \
  >> "${NEW_OPERATOR_YAML}"
  mv "${NEW_OPERATOR_YAML}" "${OPERATOR_YAML}"
}

replaceTag() {
    echo "${1}" | sed -e "s/\(.*:\).*/\1${2}/"
}

updateVersionFile() {
  echo "[INFO] updating version.go file"
  # change version/version.go file
  sed -i version/version.go -r -e 's#(Version = ")([0-9.]+)(")#\1'"${RELEASE}"'\3#g'
  git add version/version.go
  git commit -m "Update VERSION to $RELEASE" --signoff
}

releaseOlmFiles() {
  echo "[INFO] releaseOlmFiles :: Release OLM files"
  echo "[INFO] releaseOlmFiles :: Launch 'olm/release-olm-files.sh' script"
  cd $RELEASE_DIR/olm
  . release-olm-files.sh $RELEASE
  cd $RELEASE_DIR

  local openshift=$RELEASE_DIR/deploy/olm-catalog/stable/eclipse-che-preview-openshift/manifests
  local kubernetes=$RELEASE_DIR/deploy/olm-catalog/stable/eclipse-che-preview-kubernetes/manifests

  echo "[INFO] releaseOlmFiles :: Validate changes"
  grep -q "version: "$RELEASE $openshift/che-operator.clusterserviceversion.yaml
  grep -q "version: "$RELEASE $kubernetes/che-operator.clusterserviceversion.yaml

  test -f $kubernetes/org_v1_che_crd.yaml
  test -f $openshift/org_v1_che_crd.yaml

  echo "[INFO] releaseOlmFiles :: Commit changes"
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Release OLM files to "$RELEASE --signoff
  fi
}

pushOlmBundlesToQuayIo() {
  echo "[INFO] releaseOperatorCode :: Login to quay.io..."
  docker login quay.io -u "${QUAY_ECLIPSE_CHE_USERNAME}" -p "${QUAY_ECLIPSE_CHE_PASSWORD}"
  echo "[INFO] Push OLM bundles to quay.io"
  . ${RELEASE_DIR}/olm/buildAndPushBundleImages.sh -c "stable" -p "kubernetes" -f "true"
  . ${RELEASE_DIR}/olm/buildAndPushBundleImages.sh -c "stable" -p "openshift" -f "true"
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

createPRToMainBranch() {
  echo "[INFO] createPRToMainBranch :: Create pull request into main branch to copy csv"
  resetChanges main
  local tmpBranch="copy-csv-to-main"
  git checkout -B $tmpBranch
  git diff refs/heads/${BRANCH}...refs/heads/${RELEASE_BRANCH} ':(exclude)deploy/operator.yaml' | git apply -3
  if git status --porcelain; then
    git add -A || true # add new generated CSV files in olm/ folder
    git commit -am "Copy "$RELEASE" csv to main" --signoff
  fi
  git push origin $tmpBranch -f
  if [[ $FORCE_UPDATE == "--force" ]]; then set +e; fi  # don't fail if PR already exists (just force push commits into it)
  hub pull-request $FORCE_UPDATE --base main --head ${tmpBranch} -m "Copy "$RELEASE" csv to main"
  set -e
}

prepareCommunityOperatorsUpdate() {
  export BASE_DIR=${RELEASE_DIR}/olm
  . "${BASE_DIR}/prepare-community-operators-update.sh" $FORCE_UPDATE
  unset BASE_DIR
}

run() {
  if [[ $CHECK_RESOURCES == "true" ]]; then
    echo "[INFO] Check if resources are up to date"
    . ${RELEASE_DIR}/.github/bin/check-resources.sh
  fi

  checkoutToReleaseBranch
  updateVersionFile
  releaseOperatorCode
  if [[ $RELEASE_OLM_FILES == "true" ]]; then
    releaseOlmFiles
  fi
}

init "$@"
echo "[INFO] Release '$RELEASE' from branch '$BRANCH'"

if [[ $RUN_RELEASE == "true" ]]; then
  run "$@"
fi

if [[ $PUSH_OLM_BUNDLES == "true" ]]; then
  pushOlmBundlesToQuayIo
fi

if [[ $PUSH_GIT_CHANGES == "true" ]]; then
  pushGitChanges
fi

if [[ $CREATE_PULL_REQUESTS == "true" ]]; then
  createPRToXBranch
  createPRToMainBranch
fi

if [[ $PREPARE_COMMUNITY_OPERATORS_UPDATE == "true" ]]; then
  prepareCommunityOperatorsUpdate
fi
