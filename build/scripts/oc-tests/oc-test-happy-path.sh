#!/bin/bash
#
# Copyright (c) 2019-2026 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

################################ !!!   IMPORTANT   !!! ################################
########### THIS JOB USE openshift ci operators workflows to run  #####################
#######################################################################################

set -e

export DEVWORKSPACE_HAPPY_PATH="https://raw.githubusercontent.com/eclipse/che/main/tests/devworkspace-happy-path"
export OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")

runTests() {
  . "${OPERATOR_REPO}/build/scripts/oc-tests/oc-test.sh" --from sources=next

  export HAPPY_PATH_USERSTORY=SmokeTest
  export HAPPY_PATH_SUITE=test
  export MOCHA_DIRECTORY='.'
  export TS_SELENIUM_START_WORKSPACE_TIMEOUT=1080000

  bash <(curl -s ${DEVWORKSPACE_HAPPY_PATH}/remote-launch.sh)
}

runTests
