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

set -ex

export DEVWORKSPACE_HAPPY_PATH="https://raw.githubusercontent.com/eclipse/che/main/tests/devworkspace-happy-path"
export OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTests() {
  . ${OPERATOR_REPO}/build/scripts/olm/test-catalog-from-sources.sh --verbose

  export HAPPY_PATH_USERSTORY=SmokeTest
  export HAPPY_PATH_SUITE=test
  export MOCHA_DIRECTORY='.'
  bash <(curl -s ${DEVWORKSPACE_HAPPY_PATH}/remote-launch.sh)
}

runTests
