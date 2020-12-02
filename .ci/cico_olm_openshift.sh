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
set -x
set -u

# Component is defined in Openshift CI job configuration. See: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v4.yaml#L8
export CI_COMPONENT="che-operator-catalog"
export CATALOG_SOURCE_IMAGE_NAME=${CI_COMPONENT}:stable

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh

trap "catchFinish" EXIT SIGINT

# run function run the tests in ci of custom catalog source.
function runTests() {
    # see olm.sh
    export OAUTH="false"

    # Execute test catalog source script
    source "${OPERATOR_REPO}"/olm/testCatalogSource.sh "openshift" "nightly" ${NAMESPACE} "catalog" "che-catalog"
    oc project ${NAMESPACE}
    startNewWorkspace
    waitWorkspaceStart
}

init
runTests
