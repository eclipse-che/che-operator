#!/bin/bash
#
# Script to migrate OLM package based bundle to opm bundle
#
# Todo, looks like we need to need to have only one crd file for whole opm bundle, or we need support versioning for Che...

BASE_DIR=$(cd "$(dirname "$0")"; pwd) || exit 1

moveFiles() {
    platform="${1}"

    folders=($(ls -d */))

    for folder in "${folders[@]}"
    do
        version="v${folder%/}"

        pushd "${folder}" || exit 1
        if [ -f "${BASE_DIR}/${folder}eclipse-che-preview-${platform}.crd.yaml" ]; then
            mv "eclipse-che-preview-${platform}.crd.yaml" "eclipse-che-preview-${platform}.${version}.crd.yaml"
        fi
        if [ -f "${BASE_DIR}/${folder}eclipse-che-preview-${platform}.crd.yaml.diff" ]; then
            mv "eclipse-che-preview-${platform}.crd.yaml.diff" "eclipse-che-preview-${platform}.${version}.crd.yaml.diff"
        fi
        mv ./* ../
        popd || exit 1
        rm -rf "${folder}"
    done
}

k8sPlatform="kubernetes"
ocpPlatform="openshift"

pushd "eclipse-che-preview-${k8sPlatform}/deploy/olm-catalog/eclipse-che-preview-${k8sPlatform}" || exit 1
moveFiles k8sPlatform
popd || exit 1

pushd "eclipse-che-preview-${ocpPlatform}/deploy/olm-catalog/eclipse-che-preview-${ocpPlatform}" || exit 1
moveFiles ocpPlatform
popd || exit 1
