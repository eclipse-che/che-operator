#!/bin/bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
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

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTests() {
  . "${OPERATOR_REPO}/build/scripts/olm/test-update.sh" -c stable -i quay.io/eclipse/eclipse-che-olm-catalog:stable --verbose
}

runTests
