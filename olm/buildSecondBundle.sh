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

platform=kubernetes

DOCKER_USERNAME=aandriienko
IMAGE_REGISTRY=quay.io

SCRIPT=$(readlink -f "$0")
BASE_DIR=$(dirname "$SCRIPT")
ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")

OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
CSV_F="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

nightlyVersion=$(yq -r ".spec.version" "${CSV_F}")

source ${BASE_DIR}/olm.sh "${platform}" "${nightlyVersion}" "che"

echo "${nightlyVersion}"

installOPM

${BASE_DIR}/update-nightly-olm-files.sh
platform=kubernetes
nightlyVersion=$(yq -r ".spec.version" "${CSV_F}")
CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${IMAGE_REGISTRY}/${DOCKER_USERNAME}/eclipse-che-${platform}-opm-bundles:${nightlyVersion}"

buildBundleImage "${OPM_BUNDLE_MANIFESTS_DIR}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

CATALOG_IMAGENAME="${IMAGE_REGISTRY}/${DOCKER_USERNAME}/eclipse-che-${platform}-opm-catalog:0.0.1"

${OPM_BINARY} index add \
    --from-index "${CATALOG_IMAGENAME}" \
    --bundles "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --tag "${CATALOG_IMAGENAME}" \
    --build-tool docker \
    --mode semver

echo "====================Done 1"

docker push "${CATALOG_IMAGENAME}"
