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
	echo "Usage:   $0 -p [platform] -c [channel] -i [from-index-image(optional)] -f [force-build-and-push(optional)]"
	echo "Example: $0 -p openshift -c nightly -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:preview -f true"
}

if [[ $# -lt 1 ]]; then usage; exit; fi

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-c') channel="$2"; shift 1;;
    '-p') platform="$2"; shift 1;;
    '-f') forceBuildAndPush="$2"; shift 1;;
    '-i') fromIndexImage="$2"; shift 1;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! ${channel} ]] || [[ ! ${platform} ]]; then usage; exit 1; fi

if [ -z "${forceBuildAndPush}" ]; then
  forceBuildAndPush="false"
fi
if [ ! "${forceBuildAndPush}" == "true" ] && [ ! "${forceBuildAndPush}" == "false"  ]; then
  echo "[ERROR] -f argument should be 'true' or 'false'"
  exit 1
fi

if [ -z "${IMAGE_REGISTRY_HOST}" ] || [ -z "${IMAGE_REGISTRY_USER_NAME}" ]; then
    echo "[ERROR] Specify env variables with information about image registry 'IMAGE_REGISTRY_HOST' and 'IMAGE_REGISTRY_USER_NAME'."
    exit 1
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
if [[ -z "$CHECK_BUNDLE_TAG" ]] || [[ "${forceBuildAndPush}" == "true" ]]; then
  buildBundleImage "${platform}" "${CATALOG_BUNDLE_IMAGE}" "${channel}" "docker"

  if [ -n "${fromIndexImage}" ]; then
    buildCatalogImage "${CATALOG_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${forceBuildAndPush}"  "${fromIndexImage}"
    echo "[INFO] Done."
    exit 0
  fi

  CHECK_CATALOG_TAG=$(skopeo inspect docker://${CATALOG_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CATALOG_TAG}\")")
  if [ -z "${CHECK_CATALOG_TAG}" ]; then
    buildCatalogImage "${CATALOG_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${forceBuildAndPush}"
  else
    buildCatalogImage "${CATALOG_IMAGE}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${forceBuildAndPush}" "${CATALOG_IMAGE}"
  fi
else
    echo "[INFO] Bundle ${CATALOG_BUNDLE_IMAGE} is already pushed to the image registry"
fi

echo "[INFO] Done."
