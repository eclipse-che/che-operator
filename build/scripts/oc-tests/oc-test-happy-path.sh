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

export OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

export DEVWORKSPACE_HAPPY_PATH="https://raw.githubusercontent.com/eclipse/che/main/tests/devworkspace-happy-path"
source <(curl -s ${DEVWORKSPACE_HAPPY_PATH}/common.sh)

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTests() {
  . ${OPERATOR_REPO}/build/scripts/olm/test-catalog-from-sources.sh --verbose
  bash <(curl -s ${DEVWORKSPACE_HAPPY_PATH}/remote-launch.sh)
}

runTests
