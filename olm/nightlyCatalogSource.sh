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
# platform=$1
# if [ "${platform}" == "" ]; then
#   echo "Please specify platform ('openshift' or 'kubernetes') as the first argument."
#   echo ""
# #   echo "testUpdate.sh <platform> [<channel>] [<namespace>]"
#   exit 1
# fi

fromVersion=$2

BASE_DIR=$(cd "$(dirname "$0")"; pwd)
ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")

OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

nightlyVersion=$(yq -r ".spec.version" "${CSV}")


source ${BASE_DIR}/olm.sh "${platform}" "${nightlyVersion}" "che"

CATALOG_BUNDLE_IMAGE_NAME_LOCAL="docker.io/aandrienko/eclipse-che-operator-${platform}-bundle:${nightlyVersion}"

echo ${nightlyVersion}

installOPM

buildBundleImage

${BASE_DIR}/incrementNightlyBundles.sh
incrementNightlyVersion

platform=kubernetes
nightlyVersion=$(yq -r ".spec.version" "${CSV}")
CATALOG_BUNDLE_IMAGE_NAME_LOCAL="docker.io/aandrienko/eclipse-che-operator-${platform}-bundle:${nightlyVersion}"

buildBundleImage

CATALOG_IMAGENAME="docker.io/aandrienko/eclipse-che-catalog-source:0.0.1"

${OPM_BINARY} index add \
    --bundles "docker.io/aandrienko/eclipse-che-operator-${platform}-bundle:7.16.2-0.nightly" \
    --tag "${CATALOG_IMAGENAME}" \
    --build-tool docker \
    --mode semver

echo "====================Done 1"

docker push "${CATALOG_IMAGENAME}"

${OPM_BINARY} index add \
    --from-index "${CATALOG_IMAGENAME}" \
    --bundles "docker.io/aandrienko/eclipse-che-operator-${platform}-bundle:7.16.2-1.nightly" \
    --tag "${CATALOG_IMAGENAME}" \
    --build-tool docker \
    --mode semver

docker push "${CATALOG_IMAGENAME}"

echo "====================Done 2"

# --skip-tls # local registry launched without https
# --from-index  "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \


