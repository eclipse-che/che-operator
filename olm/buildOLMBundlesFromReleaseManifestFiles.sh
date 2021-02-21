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

SCRIPT=$(readlink -f "$0")
OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")")
echo "${OPERATOR_REPO}"

set -e
BASE_DIR="${OPERATOR_REPO}/olm"
source "${BASE_DIR}/olm.sh"
installOPM

for platform in 'kubernetes' 'openshift'
do
    manifestsFormatRootFolder="${OPERATOR_REPO}/olm/eclipse-che-preview-${platform}/deploy/olm-catalog/eclipse-che-preview-${platform}"
    pushd "${manifestsFormatRootFolder}" || exit 1

    initOLMScript "${platform}"
    stableBundleDir=$(getBundlePath "stable")
    echo "${stableBundleDir}"
    bundle_dir=$(mktemp -d -t che-releases-XXX)
    echo "${bundle_dir}"

    readarray -t dirs < <(find . -maxdepth 1 -type d -printf '%P\n' | sort)
    for versionDir in ${dirs[*]} ; do
        if [[ "${versionDir}" =~ [0-9]+\.[0-9]+\.[0-9]+ ]]; then
            echo "Converting manifest format folder ${versionDir} to the bundle format..."
            manifestFormatDir="${manifestsFormatRootFolder}/${versionDir}"
            bundleDir="${bundle_dir}/${versionDir}"
            mkdir -p "${bundleDir}/manifests"
            cp -rf "${stableBundleDir}/bundle.Dockerfile" "${stableBundleDir}/metadata" "${bundleDir}"
            packageName=$(getPackageName)

            cp -rf "${manifestFormatDir}/${packageName}.v${versionDir}.clusterserviceversion.yaml" "${bundleDir}/manifests/che-operator.clusterserviceversion.yaml"
            cp -rf "${manifestFormatDir}/${packageName}.crd.yaml" "${bundleDir}/manifests/org_v1_che_crd.yaml" 
            cp -rf "${manifestFormatDir}/${packageName}.v${versionDir}.clusterserviceversion.yaml.diff" "${bundleDir}/manifests/che-operator.clusterserviceversion.yaml.diff"
            cp -rf "${manifestFormatDir}/${packageName}.crd.yaml.diff" "${bundleDir}/manifests/org_v1_che_crd.yaml.diff"
        fi
    done

    for versionDir in ${dirs[*]} ; do
        if [[ "${versionDir}" =~ [0-9]+\.[0-9]+\.[0-9]+ ]]; then
            OPM_BUNDLE_DIR="${bundle_dir}/${versionDir}"
            export OPM_BUNDLE_DIR
            OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
            export OPM_BUNDLE_MANIFESTS_DIR
            CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

            stableVersion=$(yq -r ".spec.version" "${CSV}")
            echo "Preparing stable bundle version: ${stableVersion}"

            "${OPERATOR_REPO}/olm/push-catalog-and-bundle-images.sh" "stable" "${platform}"

            # CATALOG_BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-bundles:${stableVersion}"
            # echo "[INFO] Build bundle image: ${CATALOG_BUNDLE_IMAGE}"
            # buildBundleImage "${CATALOG_BUNDLE_IMAGE}" "stable" 

            # CATALOG_IMAGENAME="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${platform}-opm-catalog:preview"
            # echo "[INFO] Build CatalogSource image: ${CATALOG_IMAGENAME}"
            # buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE}" "docker" "${CATALOG_IMAGENAME}"
        fi
    done

    popd || true
done
