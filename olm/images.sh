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

setImagesFromDeploymentEnv() {
    REQUIRED_IMAGES=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.value) | select(.name | test("RELATED_IMAGE_.*"; "g")) | .value' "${CSV}" | sort | uniq)
}

setOperatorImage() {
    OPERATOR_IMAGE=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[0].image' "${CSV}")
}

setPluginRegistryList() {
    registry=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.name | test("RELATED_IMAGE_.*plugin_registry"; "g")) | .value' "${CSV}")
    setRegistryImages "${registry}"

    PLUGIN_REGISTRY_LIST=${registryImages}
}

setDevfileRegistryList() {
    registry=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.name | test("RELATED_IMAGE_.*devfile_registry"; "g")) | .value' "${CSV}")

    setRegistryImages "${registry}"
    DEVFILE_REGISTRY_LIST=${registryImages}
}

setRegistryImages() {
    registry="${1}"
    registry="${registry/\@sha256:*/:${IMAGE_TAG}}" # remove possible existing @sha256:... and use current tag instead

    echo -n "[INFO] Pull container ${registry} ..."
    ${PODMAN} pull ${registry} ${QUIET}

    registryImages="$(${PODMAN} run --rm  --entrypoint /bin/sh "${registry}" -c "cat /var/www/html/*/external_images.txt")"
    echo "[INFO] Found $(echo "${registryImages}" | wc -l) images in registry"
}
