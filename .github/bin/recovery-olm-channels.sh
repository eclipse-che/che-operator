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

if [ -n "${GITHUB_WORKSPACE}" ]; then
  ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
else
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  ROOT_PROJECT_DIR=$(dirname $(dirname "$(dirname "$SCRIPT")"))
fi

if [ -z "${IMAGE_REGISTRY_HOST}" ] || [ -z "${IMAGE_REGISTRY_USER_NAME}" ]; then
    echo "[ERROR] Specify env variables with information about image registry 'IMAGE_REGISTRY_HOST' and 'IMAGE_REGISTRY_USER_NAME'."
    exit 1
fi

source "${BASE_DIR}/olm.sh"
installOPM
${OPM_BINARY} version

for platform in 'openshift' 'kubernetes'; do
    echo "Recovery OLM channels for ${platform}"

    CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-bundles"
    echo "[INFO] Bundle image without tag: ${CATALOG_BUNDLE_IMAGE}"

    declare -a BUNDLE_TAGS=($(skopeo list-tags docker://${CATALOG_BUNDLE_IMAGE} 2>/dev/null | jq -r ".Tags[] | @sh"))
    BUNDLE_IMAGES=""
    for tag in "${BUNDLE_TAGS[@]}"; do
        tag=$(echo "${tag}" | tr -d "'")
        BUNDLE_IMAGES="${BUNDLE_IMAGES},${CATALOG_BUNDLE_IMAGE}:${tag}"
    done
    # Remove first coma
    BUNDLE_IMAGES="${BUNDLE_IMAGES#,}"

    echo "[INFO] List bundle images: ${BUNDLE_IMAGES}"
    # Rebuild and push index image with all found bundles.
    INDEX_IMG="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-catalog:preview"
    echo "[INFO] Rebuild and push catalog source image: "
    echo "[INFO] ====================================== "
    pushd "${ROOT_PROJECT_DIR}" || true
    make catalog-build BUNDLE_IMGS="${BUNDLE_IMAGES}" CATALOG_IMG="${INDEX_IMG}"
    make catalog-push CATALOG_IMG="${INDEX_IMG}"
    popd || true
done
