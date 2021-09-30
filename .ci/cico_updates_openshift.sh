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

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}"/.github/bin/oauth-provision.sh

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

overrideDefaults() {
  export CHE_EXPOSURE_STRATEGY="single-host"
}

runTests() {
  "${OPERATOR_REPO}"/olm/testUpdate.sh "openshift" "stable" ${NAMESPACE}
  waitEclipseCheDeployed ${LAST_PACKAGE_VERSION}
  provisionOAuth
  startNewWorkspace
  waitWorkspaceStart

  # Dev Workspace controller tests
  # enableDevWorkspaceEngine
  # waitDevWorkspaceControllerStarted
  # waitEclipseCheDeployed ${LAST_PACKAGE_VERSION}

  # sleep 10s
  # createWorkspaceDevWorkspaceController
  # waitAllPodsRunning ${DEVWORKSPACE_CONTROLLER_TEST_NAMESPACE}
}

initDefaults
overrideDefaults
provisionOpenShiftOAuthUser
initStableTemplates "openshift" "stable"
runTests
