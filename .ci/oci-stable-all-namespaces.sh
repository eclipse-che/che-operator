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

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

################################ !!!   IMPORTANT   !!! ################################
########### THIS JOB USE openshift ci operators workflows to run  #####################
##########  More info about how it is configured can be found here: https://docs.ci.openshift.org/docs/how-tos/testing-operator-sdk-operators #############
######### Script which install the given bundles from stable-all-namespace https://steps.ci.openshift.org/reference/optional-operators-subscribe ##########
######## !!! Related PR in openshift CI is: https://github.com/openshift/release/pull/20610 ##############################################################
#######################################################################################################################################################


export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}"/.github/bin/oauth-provision.sh

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

overrideDefaults() {
  export DEV_WORKSPACE_ENABLE="true"
  export CHE_EXPOSURE_STRATEGY="single-host"
}

runTests() {
    # create namespace
    oc create namespace eclipse-che || true

    # Deploy Eclipse Che applying CR
    applyOlmCR
    waitEclipseCheDeployed "${LAST_PACKAGE_VERSION}"

    sleep 10s
    createWorkspaceDevWorkspaceCheOperator
    waitAllPodsRunning ${DEVWORKSPACE_CHE_OPERATOR_TEST_NAMESPACE}
}

initDefaults
overrideDefaults
provisionOpenShiftOAuthUser
initStableTemplates "openshift" "stable"
runTests
