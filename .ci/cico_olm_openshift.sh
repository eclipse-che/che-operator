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

# ENV used by Openshift CI
ARTIFACTS_DIR="/tmp/artifacts"
export ARTIFACTS_DIR

# Component is defined in Openshift CI job configuration. See: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v4.yaml#L8
CI_COMPONENT="che-operator-catalog"
export CI_COMPONENT

CATALOG_SOURCE_IMAGE_NAME=${CI_COMPONENT}:stable
export CATALOG_SOURCE_IMAGE_NAME

# This image is builded by Openshift CI and exposed to be consumed for olm tests.
#OPENSHIFT_BUILD_NAMESPACE env var exposed by Openshift CI. More info about how images are builded in Openshift CI: https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
CATALOG_SOURCE_IMAGE="che-catalog"
export CATALOG_SOURCE_IMAGE

# Choose if install Eclipse Che using an operatorsource or Custom Catalog Source
INSTALLATION_TYPE="catalog"
export INSTALLATION_TYPE

# Execute olm nightly files in openshift
PLATFORM="openshift"
export PLATFORM

# Test nightly olm files
CHANNEL="nightly"
export CHANNEL

# Test nightly olm files
NAMESPACE="che"
export NAMESPACE

# run function run the tests in ci of custom catalog source.
function run() {
    export OAUTH="false"
    # Execute test catalog source script
    source "${OPERATOR_REPO}"/olm/testCatalogSource.sh ${PLATFORM} ${CHANNEL} ${NAMESPACE} ${INSTALLATION_TYPE} ${CATALOG_SOURCE_IMAGE}

    source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
    oc project ${NAMESPACE}

    # Create and start a workspace
    getCheAcessToken
    chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    getCheAcessToken
    chectl workspace:list
    waitWorkspaceStart
}

run

# grab che-operator namespace events after running olm nightly tests
oc get events -n ${NAMESPACE} | tee ${ARTIFACTS_DIR}/che-operator-events.log
