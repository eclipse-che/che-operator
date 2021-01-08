#!/bin/bash
#
# Copyright (c) 2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e
set -x

# Get absolute path for root repo directory from github actions context: https://docs.github.com/en/free-pro-team@latest/actions/reference/context-and-expression-syntax-for-github-actions
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

prepareTemplates() {
  disableOpenShiftOAuth ${TEMPLATES}
  disableUpdateAdminPassword ${TEMPLATES}
  setCustomOperatorImage ${TEMPLATES} ${OPERATOR_IMAGE}
}

runTest() {
  deployEclipseChe "operator" "minishift" ${OPERATOR_IMAGE} ${TEMPLATES}
  startNewWorkspace
  waitWorkspaceStart
}

init
installYq
initLatestTemplates
prepareTemplates
# build is done on previous github action step
copyCheOperatorImageToMinishift
runTest
