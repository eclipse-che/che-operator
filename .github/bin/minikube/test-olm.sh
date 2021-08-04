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

# Get absolute path for root repo directory from github actions context: https://docs.github.com/en/free-pro-team@latest/actions/reference/context-and-expression-syntax-for-github-actions
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$SCRIPT")")")")
fi
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}/olm/olm.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  export OPERATOR_IMAGE="${IMAGE_REGISTRY_HOST}/operator:test"
  source "${OPERATOR_REPO}"/olm/testCatalogSource.sh "kubernetes" "next" "${NAMESPACE}"
  startNewWorkspace
  waitWorkspaceStart

  # stop workspace to clean up resources
  stopExistedWorkspace
  waitExistedWorkspaceStop
  kubectl delete namespace ${USER_NAMEPSACE}

  deployCertManager

  # Dev Workspace controller tests
  enableDevWorkspaceEngine
  waitDevWorkspaceControllerStarted

  sleep 10s
  createWorkspaceDevWorkspaceController
  waitAllPodsRunning ${DEVWORKSPACE_CONTROLLER_TEST_NAMESPACE}
}

initDefaults
installOperatorMarketPlace
insecurePrivateDockerRegistry
runTest
