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

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")

init() {
  FORCE="false"
  MULTI_ARCH="false"
  IMAGE_TOOL="docker"

  unset CHANNEL
  unset CATALOG_IMAGE
  unset BUNDLE_IMAGE
  unset IMAGE_TOOL

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--bundle-image'|'-b') BUNDLE_IMAGE="$2"; shift 1;;
      '--image-tool'|'-t') IMAGE_TOOL="$2"; shift 1;;
      '--force'|'-f') FORCE="true";;
      '--multi-arch'|'-m') MULTI_ARCH="true";;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  if [[ ! ${CHANNEL} ]]; then usage; exit 1; fi

  BUNDLE_NAME=$(make bundle-name CHANNEL="${CHANNEL}")
  BUNDLE_VERSION=$(make bundle-version CHANNEL="${CHANNEL}")
  BUNDLE_IMAGE="${BUNDLE_IMAGE:=quay.io/eclipse/eclipse-che-olm-bundle:${BUNDLE_VERSION}}"
  CATALOG_IMAGE=${CATALOG_IMAGE:=quay.io/eclipse/eclipse-che-olm-catalog:${CHANNEL}}

  echo "[INFO] Bundle name   : ${BUNDLE_NAME}"
  echo "[INFO] Bundle version: ${BUNDLE_VERSION}"
  echo "[INFO] Bundle image  : ${BUNDLE_IMAGE}"
  echo "[INFO] Catalog image : ${CATALOG_IMAGE}"
}

usage () {
  echo "Build and push catalog and bundle images."
  echo
	echo "Usage:"
	echo -e "\t$0 -i CATALOG_IMAGE -c CHANNEL [-i CATALOG_IMAGE] [-b BUNDLE_IMAGE] [-t IMAGE_TOOL] [--force] [--multi-arch]"
  echo
  echo "Options:"
  echo -e "\t-c,--channel             (next or stable) Olm channel to build bundle from"
  echo -e "\t-i,--catalog-image       Catalog image to build"
  echo -e "\t-b,--bundle-image        Bundle image to build"
  echo -e "\t-t,--image-tool          [default: docker] Image tool"
  echo -e "\t-m,--multi-arch          [default: false] Build multi-arch images"
  echo -e "\t-f,--force               [default: false] Force to build catalog and bundle images even if bundle already exists in the catalog"
  echo
	echo "Example:"
	echo -e "\t$0 -c next"
	echo -e "\t$0 -c stable"
}

build () {
  CHANNEL_PATH=$(make channel-path CHANNEL="${CHANNEL}")

  make download-opm
  if [[ $(bin/opm render "${CATALOG_IMAGE}" | jq 'select (.schema == "olm.channel") | .entries[] | select(.name == "'${BUNDLE_NAME}'")') ]] && [[ "${FORCE}" == "false" ]]; then
    echo "[INFO] Bundle ${BUNDLE_NAME} already exists in the catalog"
    exit 0
  else
    echo "[INFO] Build and push the new bundle image"
    if [[ "${MULTI_ARCH}" == "true" ]]; then
      make bundle-build-and-push-multiarch \
          CHANNEL="${CHANNEL}" \
          BUNDLE_IMG="${BUNDLE_IMAGE}" \
          IMAGE_TOOL="${IMAGE_TOOL}" \
          ARCHS="linux/arm64,linux/amd64"
    else
      make bundle-build bundle-push \
          CHANNEL="${CHANNEL}" \
          BUNDLE_IMG="${BUNDLE_IMAGE}" \
          IMAGE_TOOL="${IMAGE_TOOL}"
    fi

    echo "[INFO] Add bundle to the catalog"

    BUNDLE_IMAGE_INSPECT=$(skopeo inspect docker://${BUNDLE_IMAGE})
    BUNDLE_IMAGE_WITH_DIGESTS=$(echo "${BUNDLE_IMAGE_INSPECT}" | jq -r '.Name')@$(echo "${BUNDLE_IMAGE_INSPECT}" | jq -r '.Digest')

    echo "[INFO] Build image with digest: ${BUNDLE_IMAGE_WITH_DIGESTS}"

    # Reference to the bundle image with digest instead of tag
    # to deploy Eclipse Che in disconnected cluster
    make bundle-render CHANNEL="${CHANNEL}" BUNDLE_NAME="${BUNDLE_NAME}" BUNDLE_IMG="${BUNDLE_IMAGE_WITH_DIGESTS}"

    LAST_BUNDLE_NAME=$(yq -r '.entries | .[length - 1].name' "${CHANNEL_PATH}")
    if [[ ${CHANNEL} == "stable" ]]; then
      yq -riY '(.entries) += [{"name": "'${BUNDLE_NAME}'", "replaces": "'${LAST_BUNDLE_NAME}'"}]' "${CHANNEL_PATH}"
    else
      yq -riY '(.entries) = [{"name": "'${BUNDLE_NAME}'", "skipRange": "<'${BUNDLE_VERSION}'"}]' "${CHANNEL_PATH}"
    fi
  fi

  echo "[INFO] Build and push the catalog image"
  if [[ "${MULTI_ARCH}" == "true" ]]; then
    make catalog-build-and-push-multiarch \
      CHANNEL="${CHANNEL}" \
      CATALOG_IMG="${CATALOG_IMAGE}" \
      IMAGE_TOOL="${IMAGE_TOOL}" \
      ARCHS="linux/arm64,linux/amd64"
    else
      make catalog-build catalog-push \
        CHANNEL="${CHANNEL}" \
        CATALOG_IMG="${CATALOG_IMAGE}" \
        IMAGE_TOOL="${IMAGE_TOOL}"
    fi

  make download-addlicense
  make license $(make catalog-path CHANNEL="${CHANNEL}")
}

init "$@"

pushd "${OPERATOR_REPO}" >/dev/null
build
popd >/dev/null

echo "[INFO] Done"
