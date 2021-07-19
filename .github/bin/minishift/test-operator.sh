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
if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$SCRIPT")")")")
fi
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

patchTemplates() {
  disableOpenShiftOAuth ${TEMPLATES}
  disableUpdateAdminPassword ${TEMPLATES}
  setCustomOperatorImage ${TEMPLATES} ${OPERATOR_IMAGE}
}

runTest() {
  deployEclipseCheWithTemplates "operator" "minishift" ${OPERATOR_IMAGE} ${TEMPLATES}
  startNewWorkspace
  waitWorkspaceStart
}

initDefaults
installYq
initLatestTemplates
patchTemplates
if [[ -z "$GITHUB_ACTIONS" ]]; then
  buildCheOperatorImage
fi
copyCheOperatorImageToMinishift
runTest
