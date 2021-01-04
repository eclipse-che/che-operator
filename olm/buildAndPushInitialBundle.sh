#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

printHelp() {
  echo ''
  echo 'Please consider to pass this values to the script to run script:'
	echo '    PLATFORM                 - cluster platform: "kubernetes" or "openshift".'
    echo '    FROM_INDEX_IMAGE         - (Optional) Using this argument you can include Olm bundles from another index image to you index(CatalogSource) image'
  echo ''
  echo 'EXAMPLE of running: ${OPERATOR_REPO}/olm/buildAndPushInitialBundle.sh openshift'
}

PLATFORM="${1}"
if [ "${PLATFORM}" == "" ]; then
  echo -e "${RED}[ERROR]: Please specify a valid platform. The posible platforms are kubernetes or openshift.The script will exit with code 1.${NC}"
  printHelp
  exit 1
else
  echo "[INFO]: Successfully validated platform. Starting olm tests in platform: ${PLATFORM}."
fi

FROM_INDEX_IMAGE="${2}"

if [ -z "${IMAGE_REGISTRY_HOST}" ] || [ -z "${IMAGE_REGISTRY_USER_NAME}" ]; then
    echo "[ERROR] Specify env variables with information about image registry 'IMAGE_REGISTRY_HOST' and 'IMAGE_REGISTRY_USER_NAME'."
fi

SCRIPT=$(readlink -f "$0")
BASE_DIR=$(dirname "$SCRIPT")
ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")

OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/eclipse-che-preview-${PLATFORM}"
OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

nightlyVersion=$(yq -r ".spec.version" "${CSV}")

source ${BASE_DIR}/olm.sh "${PLATFORM}" "${nightlyVersion}" "che"

CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${PLATFORM}-opm-bundles:${nightlyVersion}"

echo "${nightlyVersion}"

installOPM

echo "[INFO] Build bundle image: ${CATALOG_BUNDLE_IMAGE}"
buildBundleImage "${CATALOG_BUNDLE_IMAGE}"

echo "[INFO] Build CatalogSource image: ${CATALOG_BUNDLE_IMAGE}"
CATALOG_IMAGENAME="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${PLATFORM}-opm-catalog:preview"
buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${FROM_INDEX_IMAGE}"

echo "[INFO] Done. Images '${CATALOG_IMAGENAME}' and '${CATALOG_BUNDLE_IMAGE}' were build and pushed"
