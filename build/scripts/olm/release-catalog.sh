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

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")

init() {
  unset CHANNEL
  unset CATALOG_IMAGE
  unset IMAGE_TOOL

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--image-tool'|'-t') IMAGE_TOOL="$2"; shift 1;;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  [[ ! ${IMAGE_TOOL} ]] && IMAGE_TOOL="docker"
  if [[ ! ${CHANNEL} ]] || [[ ! ${CATALOG_IMAGE} ]]; then usage; exit 1; fi

  BUNDLE_NAME=$(make bundle-name CHANNEL="${CHANNEL}")
  BUNDLE_VERSION=$(make bundle-version CHANNEL="${CHANNEL}")
  REGISTRY="$(echo "${CATALOG_IMAGE}" | rev | cut -d '/' -f2- | rev)"
  BUNDLE_IMAGE="${REGISTRY}/eclipse-che-olm-bundle:${BUNDLE_VERSION}"

  echo "[INFO] Bundle name   : ${BUNDLE_NAME}"
  echo "[INFO] Bundle version: ${BUNDLE_VERSION}"
  echo "[INFO] Bundle image  : ${BUNDLE_IMAGE}"
  echo "[INFO] Catalog image : ${CATALOG_IMAGE}"
}

usage () {
  echo "Build and push catalog and bundle images."
  echo
	echo "Usage:"
	echo -e "\t$0 -i CATALOG_IMAGE -c CHANNEL [-o OPERATOR_IMAGE] [-t IMAGE_TOOL]"
  echo
  echo "Options:"
  echo -e "\t-i,--catalog-image       Catalog image to build"
  echo -e "\t-c,--channel=next|stable Olm channel to build bundle from"
  echo -e "\t-t,--image-tool          [default: docker] Image tool"
  echo
	echo "Example:"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-olm-catalog:next -c next"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-olm-catalog:stable -c stable"
}

buildBundle() {
  echo "[INFO] Build and push the new bundle image"
  make bundle-build bundle-push CHANNEL="${CHANNEL}" BUNDLE_IMG="${BUNDLE_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}"
}

buildCatalog () {
  CHANNEL_PATH=$(make channel-path CHANNEL="${CHANNEL}")

  if [[ $(yq -r '.entries[] | select(.name == "'${BUNDLE_NAME}'") | length' "${CHANNEL_PATH}") == 1 ]]; then
    echo "[WARN] Bundle ${BUNDLE_NAME} already exists in the catalog"
  else
    echo "[INFO] Add bundle to the catalog"

    LAST_BUNDLE_NAME=$(yq -r '.entries | .[length - 1].name' "${CHANNEL_PATH}")
    make bundle-render CHANNEL="${CHANNEL}" BUNDLE_NAME="${BUNDLE_NAME}" BUNDLE_IMG="${BUNDLE_IMAGE}"
    yq -riY '(.entries) += [{"name": "'${BUNDLE_NAME}'", "replaces": "'${LAST_BUNDLE_NAME}'"}]' "${CHANNEL_PATH}"
  fi

  echo "[INFO] Build and push the catalog image"
  make catalog-build catalog-push CHANNEL="${CHANNEL}" CATALOG_IMG="${CATALOG_IMAGE}" IMAGE_TOOL="${IMAGE_TOOL}"

  make license $(make catalog-path CHANNEL="${CHANNEL}")
}

init "$@"

pushd "${OPERATOR_REPO}" >/dev/null
buildBundle
buildCatalog
popd >/dev/null

echo "[INFO] Done"
