#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -ex

usage () {
	echo "Usage:   $0 -p [platform] -c [channel]"
	echo "Example: ./olm/buildAndPushBundle.sh -c nightly -i ${FROM_INDEX_IMAGE}"
}

if [[ $# -lt 1 ]]; then usage; exit; fi

platforms=()
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-c') channel="$2"; shift 1;;
    '-p') platforms+=("$2"); shift 1;;
    '-i') fromIndexImage="$2"; shift 1;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [ -z "${IMAGE_REGISTRY_HOST}" ] || [ -z "${IMAGE_REGISTRY_USER_NAME}" ]; then
    echo "[ERROR] Specify env variables with information about image registry 'IMAGE_REGISTRY_HOST' and 'IMAGE_REGISTRY_USER_NAME'."
fi

if [ -n "${GITHUB_WORKSPACE}" ]; then
  ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
else
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  ROOT_PROJECT_DIR=$(dirname "$(dirname "$SCRIPT")")
fi

export BASE_DIR="${ROOT_PROJECT_DIR}/olm"

source "${BASE_DIR}/olm.sh"
installOPM
${OPM_BINARY} version

for platform in "${platforms[@]}"
do
  echo "[INFO] Platform: ${platform}"
  if [ -n "${OPM_BUNDLE_DIR}" ]; then
    bundleDir="${OPM_BUNDLE_DIR}"
  else
    bundleDir=$(getBundlePath "${platform}" "${channel}")
  fi
  OPM_BUNDLE_MANIFESTS_DIR="${bundleDir}/manifests"
  CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

  BUNDLE_TAG=$(yq -r ".spec.version" "${CSV}")
  echo "[INFO] Bundle version and tag: ${BUNDLE_TAG}"

  CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-bundles:${BUNDLE_TAG}"
  CATALOG_TAG="preview"
  CATALOG_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-catalog:${CATALOG_TAG}"

  CHECK_BUNDLE_TAG=$(skopeo inspect docker://${CATALOG_BUNDLE_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${BUNDLE_TAG}\")")
  if [ -z "$CHECK_BUNDLE_TAG" ]; then
    buildBundleImage "${platform}" "${CATALOG_BUNDLE_IMAGE}" "${channel}" "docker"

    if [ -n "${fromIndexImage}" ]; then
      buildCatalogImage "${CATALOG_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${fromIndexImage}"
      continue
    fi

    CHECK_CATALOG_TAG=$(skopeo inspect docker://${CATALOG_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CATALOG_TAG}\")")
    if [ -z "${CHECK_CATALOG_TAG}" ]; then
      buildCatalogImage "${CATALOG_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker"
    else
      buildCatalogImage "${CATALOG_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${CATALOG_IMAGE}"
    fi
  else
      echo "[INFO] Bundle ${CATALOG_BUNDLE_IMAGE} is already pushed to the image registry"
  fi
done
