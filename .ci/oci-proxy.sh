#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

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
}

runTests() {
    # Deploy Eclipse Che behind proxy using chectl cli
    deployCheBehindProxy
    provisionOAuth
    startNewWorkspace
    waitWorkspaceStart
}

initDefaults
overrideDefaults
provisionOpenShiftOAuthUser
runTests
