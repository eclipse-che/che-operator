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
  local channel
  local catalogImage

  if [[ $GITHUB_HEAD_REF =~ release$ ]]; then
    channel=stable
    catalogImage=quay.io/eclipse/eclipse-che-kubernetes-opm-catalog:test
  else
    # build operator image and push to a local docker registry
    export OPERATOR_IMAGE="127.0.0.1:5000/test/operator:test"
    buildAndPushCheOperatorImage

    # build catalog source
    channel=next
    catalogImage=127.0.0.1:5000/test/catalog:test
    "${OPERATOR_REPO}"/olm/buildCatalog.sh -p kubernetes -c next -i ${catalogImage} -o ${OPERATOR_IMAGE}
  fi

  source "${OPERATOR_REPO}"/olm/testCatalog.sh -p kubernetes -c ${channel} -n ${NAMESPACE} -i ${catalogImage}
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

  # sleep 10s
  # createWorkspaceDevWorkspaceController
  # waitAllPodsRunning ${DEVWORKSPACE_CONTROLLER_TEST_NAMESPACE}
}

initDefaults
runTest
