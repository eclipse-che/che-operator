#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

set -e
# Detect the base directory where che-operator is cloned
SCRIPT=$(readlink -f "$0")
export SCRIPT

OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")");
export OPERATOR_REPO

# Container image name of Catalog source
CATALOG_SOURCE_IMAGE=my_image
export CATALOG_SOURCE_IMAGE

# Choose if install Eclipse Che using an operatorsource or Custom Catalog Source
INSTALLATION_TYPE="catalog"
export INSTALLATION_TYPE

# Execute olm nightly files in openshift
PLATFORM="kubernetes"
export PLATFORM

# Test nightly olm files
CHANNEL="nightly"
export CHANNEL

# Test nightly olm files
NAMESPACE="che"
export NAMESPACE

# Operator image
OPERATOR_IMAGE="quay.io/eclipse/che-operator:nightly"
export OPERATOR_IMAGE

IMAGE_REGISTRY_HOST="0.0.0.0:5000"
export IMAGE_REGISTRY_HOST

# run function run the tests in ci of custom catalog source.
function run() {
    # Execute test catalog source script
    source "${OPERATOR_REPO}"/olm/testCatalogSource.sh ${PLATFORM} ${CHANNEL} ${NAMESPACE} ${INSTALLATION_TYPE} ${CATALOG_SOURCE_IMAGE}

    source "${OPERATOR_REPO}"/.ci/util/ci_common.sh

    # Create and start a workspace
    getCheAcessToken
    chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    getCheAcessToken
    chectl workspace:list
    waitWorkspaceStart
}

function setPrivateRegistryForDocker {
    dockerDaemonConfig="/etc/docker/daemon.json"
    mkdir -p "/etc/docker"
    touch "${dockerDaemonConfig}"

    config="{\"insecure-registries\" : [\"${IMAGE_REGISTRY_HOST}\"]}"
    echo "${config}" | sudo tee "${dockerDaemonConfig}"

    if [ -x "$(command -v docker)" ]; then
        echo "[INFO] Restart docker daemon to set up private registry info."
        systemctl restart docker
    fi
}

source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
installYQ
installJQ
install_VirtPackages
# Docker should trust minikube private registry provided by "registry" addon
setPrivateRegistryForDocker
installStartDocker

source ${OPERATOR_REPO}/.ci/start-minikube.sh
installChectl
run
