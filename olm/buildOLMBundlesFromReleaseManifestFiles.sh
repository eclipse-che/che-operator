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

set -e
set -x

unset PLATFORM
unset FROM_INDEX_IMAGE

SCRIPT=$(readlink -f "$0")
OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")")
BASE_DIR="${OPERATOR_REPO}/olm"
source "${BASE_DIR}/olm.sh"

usage () {
	echo "Usage:   $0 -p platform [-i from-index-image]"
	echo "Example: $0 -p openshift -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:preview"
}

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-p') PLATFORM="$2"; shift 1;;
    '-i') FROM_INDEX_IMAGE="$2"; shift 1;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

run() {
    manifestsFormatRootFolder="${OPERATOR_REPO}/olm/eclipse-che-preview-${PLATFORM}/deploy/olm-catalog/eclipse-che-preview-${PLATFORM}"
    pushd "${manifestsFormatRootFolder}" || exit 1

    stableBundleDir=$(getBundlePath "${PLATFORM}" "stable")
    echo "[INFO] Stable bundle directory: ${stableBundleDir}"
    bundle_dir=$(mktemp -d -t che-releases-XXX)
    echo "[INFO] Bundle directory ${bundle_dir}"

    readarray -t dirs < <(find . -maxdepth 1 -type d -printf '%P\n' | sort)
    for versionDir in ${dirs[*]} ; do
        if [[ "${versionDir}" =~ [0-9]+\.[0-9]+\.[0-9]+ ]]; then
            echo "[INFO] Converting manifest format folder ${versionDir} to the bundle format..."

            manifestFormatDir="${manifestsFormatRootFolder}/${versionDir}"
            bundleDir="${bundle_dir}/${versionDir}"
            mkdir -p "${bundleDir}/manifests"
            cp -rf "${stableBundleDir}/bundle.Dockerfile" "${stableBundleDir}/metadata" "${bundleDir}"
            packageName=$(getPackageName "${PLATFORM}")

            # Copying resources to bundle directory
            cp -rf "${manifestFormatDir}/${packageName}.v${versionDir}.clusterserviceversion.yaml" "${bundleDir}/manifests/che-operator.clusterserviceversion.yaml"
            cp -rf "${manifestFormatDir}/${packageName}.crd.yaml" "${bundleDir}/manifests/org_v1_che_crd.yaml"
            cp -rf "${manifestFormatDir}/${packageName}.v${versionDir}.clusterserviceversion.yaml.diff" "${bundleDir}/manifests/che-operator.clusterserviceversion.yaml.diff"
            cp -rf "${manifestFormatDir}/${packageName}.crd.yaml.diff" "${bundleDir}/manifests/org_v1_che_crd.yaml.diff"

            OPM_BUNDLE_DIR="${bundle_dir}/${versionDir}"
            export OPM_BUNDLE_DIR

            # Build and push images
            "${OPERATOR_REPO}/olm/buildAndPushBundleImages.sh" -c "stable" -p $PLATFORM -i $FROM_INDEX_IMAGE
        fi
    done

    popd || true
}

installOPM
run
