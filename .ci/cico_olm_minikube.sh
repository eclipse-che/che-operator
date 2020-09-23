#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

set -ex
# Detect the base directory where che-operator is cloned
SCRIPT=$(readlink -f "$0")
export SCRIPT

OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")");
export OPERATOR_REPO

# Import operator bash utilities
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh

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

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Catch_Finish is executed after finish script.
catchFinish() {
  result=$?

  if [ "$result" != "0" ]; then
    echo "[ERROR] Please check the artifacts in github actions"
    getCheClusterLogs
    exit 1
  fi

  echo "[INFO] Job finished Successfully.Please check the artifacts in github actions"
  getCheClusterLogs

  exit $result
}

# run function run the tests in ci of custom catalog source.
function run() {
    # Execute test catalog source script
    source "${OPERATOR_REPO}"/olm/testCatalogSource.sh ${PLATFORM} ${CHANNEL} ${NAMESPACE} ${INSTALLATION_TYPE} ${CATALOG_SOURCE_IMAGE}

    # Create and start a workspace
    getCheAcessToken
    chectl workspace:create --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    getCheAcessToken
    workspaceList=$(chectl workspace:list)
    workspaceID=$(echo "$workspaceList" | grep -oP '\bworkspace.*?\b')
    chectl workspace:start $workspaceID
    waitWorkspaceStart
}

function setPrivateRegistryForDocker {
    dockerDaemonConfig="/etc/docker/daemon.json"
    sudo mkdir -p "/etc/docker"
    sudo touch "${dockerDaemonConfig}"

    config="{\"insecure-registries\" : [\"${IMAGE_REGISTRY_HOST}\"]}"
    echo "${config}" | sudo tee "${dockerDaemonConfig}"

    if [ -x "$(command -v docker)" ]; then
        echo "[INFO] Restart docker daemon to set up private registry info."
        sudo service docker restart
    fi
}

setPrivateRegistryForDocker
run
