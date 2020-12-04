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

overrideDefaults() {
  # CHE_OPERATOR_IMAGE is exposed in openshift ci pod. This image is build in every job and used then to deploy Che
  # More info about how images are builded in Openshift CI: https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  export OPERATOR_IMAGE=${CHE_OPERATOR_IMAGE}
  echo "[INFO] Che Operator Image used is: ${CHE_OPERATOR_IMAGE}"
}

prepareTemplates() {
  disableOpenShiftOAuth ${TEMPLATES}
  disableUpdateAdminPassword ${TEMPLATES}
  setCustomOperatorImage ${TEMPLATES} ${OPERATOR_IMAGE}
  setServerExposureStrategy ${TEMPLATES} "single-host"
}

runTests() {
  deployEclipseChe "operator" "openshift" ${OPERATOR_IMAGE} ${TEMPLATES}
  startNewWorkspace
  waitWorkspaceStart
}

init
overrideDefaults
initLatestTemplates
prepareTemplates
runTests
