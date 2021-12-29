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

set -e

export OPERATOR_REPO="${GITHUB_WORKSPACE}"

if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")")
fi
source "${OPERATOR_REPO}"/olm/olm.sh

init() {
  FORCE="false"
  unset CHANNEL
  unset CATALOG_IMAGE
  unset OPERATOR_IMAGE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--operator-image'|'-o') OPERATOR_IMAGE="$2"; shift 1;;
      '--force'|'-f') FORCE="true";;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  if [[ ! ${CHANNEL} ]] || [[ ! ${CATALOG_IMAGE} ]]; then usage; exit 1; fi

  BUNDLE_DIR=$(getBundlePath "${CHANNEL}")
  OPM_BUNDLE_MANIFESTS_DIR="${BUNDLE_DIR}/manifests"
  CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"
  CSV_NAME=$(yq -r ".metadata.name" "${CSV}")
  CSV_VERSION=$(yq -r ".spec.version" "${CSV}")

  IMAGE_REGISTRY_HOST=$(echo ${CATALOG_IMAGE} | cut -d '/' -f1)
  IMAGE_REGISTRY_USER_NAME=$(echo ${CATALOG_IMAGE} | cut -d '/' -f2)
  BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-openshift-opm-bundles:${CSV_VERSION}"

  echo "[INFO] CSV: ${CSV}"
  echo "[INFO] CSV name: ${CSV_NAME}"
  echo "[INFO] CSV version: ${CSV_VERSION}"
  echo "[INFO] Bundle image: ${BUNDLE_IMAGE}"

  if [[ ! -z ${OPERATOR_IMAGE} ]]; then
    # set a given operator image into CSV before build
    sed -e "s|image:.*|image: ${OPERATOR_IMAGE}|" -i "${CSV}"
  fi
}

usage () {
  echo "Build and push custom catalog and bundle images."
  echo
	echo "Usage:"
	echo -e "\t$0 -i CATALOG_IMAGE -c CHANNEL [-o OPERATOR_IMAGE] [--force]"
  echo
  echo "OPTIONS:"
  echo -e "\t-i,--catalog-image       Catalog image to build"
  echo -e "\t-c,--channel=next|stable Olm channel to build bundle from"
  echo -e "\t-o,--operator-image      Operator image to include into bundle"
  echo -e "\t-f,--force               Force to rebuild a bundle"
  echo
	echo "Example:"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -c next"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -c next -f"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -c stable"
}

buildBundle() {
  if [[ $(isBundleImageExists) == 0 ]] || [[ "${FORCE}" == "true" ]]; then
    echo "[INFO] Build bundle image"
    buildBundleImage "${BUNDLE_IMAGE}" "${CHANNEL}" "docker"
  else
    echo "[INFO] Bundle image already exists. Use '--force' flag to force build."
  fi
}

buildCatalog () {
  if [ $(isCatalogExists) == 0 ]; then
    echo "[INFO] Build a new catalog"
    buildCatalogImage "${CATALOG_IMAGE}" "${BUNDLE_IMAGE}" "docker" "${FORCE}"
  else
    if [[ $(isBundleExistsInCatalog) == 0 ]]; then
      echo "[INFO] Add a new bundle ${CSV_NAME} to the catalog"
      buildCatalogImage "${CATALOG_IMAGE}" "${BUNDLE_IMAGE}" "docker" "${FORCE}" "${CATALOG_IMAGE}"
    else
      echo "[INFO] Bundle ${CSV_NAME} already exists in the catalog"
    fi
  fi
}

isBundleExistsInCatalog() {
  local BUNDLE_NAME=$(docker run --entrypoint sh ${CATALOG_IMAGE} -c "apk add sqlite && sqlite3 /database/index.db 'SELECT operatorbundle_name FROM channel_entry WHERE channel_name=\"${CHANNEL}\" and operatorbundle_name=\"${CSV_NAME}\"'" | tail -n1 | tr -d '\r')

  # docker run produce more output then a single line
  # so, it is needed to check if the last line is actually a given bunle name
  if [[ ${BUNDLE_NAME} == ${CSV_NAME} ]]; then echo 1; else echo 0; fi
}

isBundleImageExists() {
  skopeo inspect docker://${BUNDLE_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CSV_VERSION}\")" | wc -l
}

isCatalogExists() {
  CATALOG_TAG=$(echo $CATALOG_IMAGE | rev | cut -d ':' -f1 | rev)
  skopeo inspect docker://${CATALOG_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CATALOG_TAG}\")" | wc -l
}

buildBundleImage() {
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${1}"
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "[ERROR] 'opm bundle' is not specified"
    exit 1
  fi
  channel="${2}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] 'channel' is not specified"
    exit 1
  fi
  imageTool="${3}"
  if [ -z "${imageTool}" ]; then
    echo "[ERROR] 'imageTool' is not specified"
    exit 1
  fi

  echo "[INFO] build bundle image"

  pushd "${ROOT_DIR}" || exit

  make bundle-build bundle-push channel="${channel}" BUNDLE_IMG="${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" IMAGE_TOOL="${imageTool}"
  popd || exit
}

buildCatalogImage() {
  CATALOG_IMAGENAME="${1}"
  if [ -z "${CATALOG_IMAGENAME}" ]; then
    echo "[ERROR] Please specify first argument: 'catalog image'"
    exit 1
  fi

  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${2}"
  if [ -z "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" ]; then
    echo "[ERROR] Please specify second argument: 'opm bundle image'"
    exit 1
  fi

  imageTool="${3}"
  if [ -z "${imageTool}" ]; then
    echo "[ERROR] Please specify third argument: 'image tool'"
    exit 1
  fi

  forceBuildAndPush="${4}"
  if [ -z "${forceBuildAndPush}" ]; then
    echo "[ERROR] Please specify fourth argument: 'force build and push: true or false'"
    exit 1
  fi

  # optional argument
  FROM_INDEX=${5:-""}
  BUILD_INDEX_IMAGE_ARG=""
  if [ ! "${FROM_INDEX}" == "" ]; then
    BUILD_INDEX_IMAGE_ARG=" --from-index ${FROM_INDEX}"
  fi

  SKIP_TLS_ARG=""
  SKIP_TLS_VERIFY=""
  if [ "${imageTool}" == "podman" ]; then
    SKIP_TLS_ARG=" --skip-tls"
    SKIP_TLS_VERIFY=" --tls-verify=false"
  fi

  pushd "${ROOT_DIR}" || exit

  INDEX_ADD_CMD="make catalog-build \
    CATALOG_IMG=\"${CATALOG_IMAGENAME}\" \
    BUNDLE_IMG=\"${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}\" \
    IMAGE_TOOL=\"${imageTool}\" \
    FROM_INDEX_OPT=\"${BUILD_INDEX_IMAGE_ARG}\""

  exitCode=0
  # Execute command and store an error output to the variable for following handling.
  {
    output="$(eval "${INDEX_ADD_CMD}" 2>&1 1>&3 3>&-)"; } 3>&1 || \
    {
      exitCode="$?";
      echo "[INFO] ${exitCode}";
      true;
    }
    echo "${output}"
  if [[ "${output}" == *"already exists, Bundle already added that provides package and csv"* ]] && [[ "${forceBuildAndPush}" == "true" ]]; then
    echo "[INFO] Ignore error 'Bundle already added'"
    # Catalog bundle image contains bundle reference, continue without unnecessary push operation
    return
  else
    echo "[INFO] ${exitCode}"
    if [ "${exitCode}" != 0 ]; then
      exit "${exitCode}"
    fi
  fi

  make catalog-push CATALOG_IMG="${CATALOG_IMAGENAME}"

  popd || exit
}


init $@
installOPM
buildBundle
buildCatalog

echo "[INFO] Done"
