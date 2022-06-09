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
source "${OPERATOR_REPO}/.github/bin/common.sh"

init() {
  FORCE="false"
  IMAGE_TOOL="docker"
  CHANNEL="next"
  unset CATALOG_IMAGE
  unset OPERATOR_IMAGE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--operator-image'|'-o') OPERATOR_IMAGE="$2"; shift 1;;
      '--image-tool'|'-t') IMAGE_TOOL="$2"; shift 1;;
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

  IMAGE_REGISTRY_HOST=$(echo "${CATALOG_IMAGE}" | cut -d '/' -f1)
  IMAGE_REGISTRY_USER_NAME=$(echo "${CATALOG_IMAGE}" | cut -d '/' -f2)
  BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-openshift-opm-bundles:${CSV_VERSION}"

  echo "[INFO] CSV         : ${CSV}"
  echo "[INFO] CSV name    : ${CSV_NAME}"
  echo "[INFO] CSV version : ${CSV_VERSION}"
  echo "[INFO] Bundle image: ${BUNDLE_IMAGE}"

  if [[ -n ${OPERATOR_IMAGE} ]]; then
    # set a given operator image into CSV before build
    sed -e "s|image:.*|image: ${OPERATOR_IMAGE}|" -i "${CSV}"
  fi
}

usage () {
  echo "Build and push custom catalog and bundle images."
  echo
	echo "Usage:"
	echo -e "\t$0 -i CATALOG_IMAGE [-c CHANNEL] [-o OPERATOR_IMAGE] [-t IMAGE_TOOL] [--force]"
  echo
  echo "OPTIONS:"
  echo -e "\t-i,--catalog-image       Catalog image to build"
  echo -e "\t-c,--channel=next|stable [default: next] Olm channel to build bundle from"
  echo -e "\t-o,--operator-image      Operator image to include into bundle"
  echo -e "\t-t,--image-tool          [default: docker] Image tool"
  echo -e "\t-f,--force               Force to rebuild a bundle"
  echo
	echo "Example:"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -c next"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -c next -t podman -f"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -c stable"
}

buildBundle() {
  if [[ $(isBundleImageExists) == 0 ]] || [[ "${FORCE}" == "true" ]]; then
    echo "[INFO] Build bundle image"
    buildBundleImage
  else
    echo "[INFO] Bundle image already exists. Use '--force' flag to force build."
  fi
}

buildBundleImage() {
  make bundle-build bundle-push CHANNEL="${CHANNEL}" BUNDLE_IMG="${BUNDLE_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}"
}

buildCatalog () {
  if [ $(isCatalogExists) == 0 ]; then
    echo "[INFO] Build a new catalog"
    buildCatalogImage
  else
    if [[ $(isBundleExistsInCatalog) == 0 ]]; then
      echo "[INFO] Add a new bundle ${CSV_NAME} to the catalog"
      buildCatalogImageFromIndex
    else
      echo "[INFO] Bundle ${CSV_NAME} already exists in the catalog"
    fi
  fi
}

isBundleExistsInCatalog() {
  local EXISTED_BUNDLE_NAME=$(${IMAGE_TOOL} run --entrypoint sh "${CATALOG_IMAGE}" -c "apk add sqlite && sqlite3 /database/index.db 'SELECT operatorbundle_name FROM channel_entry WHERE channel_name=\"${CHANNEL}\" and operatorbundle_name=\"${CSV_NAME}\"'" | tail -n1 | tr -d '\r')

  # docker run produce more output then a single line
  # so, it is needed to check if the last line is actually a given bundle name
  if [[ "${EXISTED_BUNDLE_NAME}" == "${CSV_NAME}" ]]; then echo 1; else echo 0; fi
}

isBundleImageExists() {
  skopeo inspect docker://"${BUNDLE_IMAGE}" 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CSV_VERSION}\")" | wc -l
}

isCatalogExists() {
  CATALOG_TAG=$(echo "${CATALOG_IMAGE}" | rev | cut -d ':' -f1 | rev)
  skopeo inspect docker://"${CATALOG_IMAGE}" 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CATALOG_TAG}\")" | wc -l
}

buildCatalogImageFromIndex() {
  make catalog-build CATALOG_IMG="${CATALOG_IMAGE}" BUNDLE_IMG="${BUNDLE_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}" FROM_INDEX_OPT="--from-index ${CATALOG_IMAGE}"
  make catalog-push CATALOG_IMG="${CATALOG_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}"
}

buildCatalogImage() {
  make catalog-build CATALOG_IMG="${CATALOG_IMAGE}" BUNDLE_IMG="${BUNDLE_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}"
  make catalog-push CATALOG_IMG="${CATALOG_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}"
}

init "$@"

pushd "${ROOT_DIR}" || exit
buildBundle
buildCatalog
popd || exit

echo "[INFO] Done"
