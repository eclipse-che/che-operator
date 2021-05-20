#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

################################ !!!   IMPORTANT   !!! ################################
########### THIS JOB USE openshift ci operators workflows to run  #####################
##########  More info about how it is configured can be found here: https://docs.ci.openshift.org/docs/how-tos/testing-operator-sdk-operators #############
#######################################################################################################################################################

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}"/.github/bin/oauth-provision.sh

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

overrideDefaults() {
  # CI_CHE_OPERATOR_IMAGE it is che operator image builded in openshift CI job workflow. More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  export OPERATOR_IMAGE=${CI_CHE_OPERATOR_IMAGE}
  export CHE_EXPOSURE_STRATEGY="single-host"
  export DEV_WORKSPACE_ENABLE="true"
}

runTests() {
    # Deploy Eclipse Che applying CR
    applyOlmCR
    waitEclipseCheDeployed "nightly"
    provisionOAuth
    startNewWorkspace
    waitWorkspaceStart

    # Dev Workspace controller tests
    waitDevWorkspaceControllerStarted

    sleep 10s
    createWorkspaceDevWorkspaceController
    waitWorkspaceStartedDevWorkspaceController

    sleep 10s
    createWorkspaceDevWorkspaceCheOperator
    waitWorkspaceStartedDevWorkspaceController
}

initDefaults
overrideDefaults
provisionOpenShiftOAuthUser
patchEclipseCheOperatorImage
printOlmCheObjects
runTests
