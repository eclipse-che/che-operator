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
set -u

export OPERATOR_REPO=$(dirname $(dirname $(dirname $(dirname $(readlink -f "$0")))))
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

prepareTemplates() {
  disableOpenShiftOAuth ${PREVIOUS_OPERATOR_TEMPLATE}
  disableOpenShiftOAuth ${LAST_OPERATOR_TEMPLATE}
}

runTest() {
  prepareTemplates

  deployEclipseChe "operator" "minishift" "quay.io/eclipse/che-operator:${PREVIOUS_PACKAGE_VERSION}" ${PREVIOUS_OPERATOR_TEMPLATE}
  sleep 5s

  createWorkspace
  sleep 5s

  updateEclipseChe "quay.io/eclipse/che-operator:${LAST_PACKAGE_VERSION}" ${LAST_OPERATOR_TEMPLATE}
  waitEclipseCheDeployed ${LAST_PACKAGE_VERSION}
  sleep 5s

  startExistedWorkspace
  waitWorkspaceStart
}

init
initStableTemplates "openshift" "stable"
runTest
