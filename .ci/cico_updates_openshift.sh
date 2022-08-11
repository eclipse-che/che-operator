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

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}/.github/bin/common.sh"
source "${OPERATOR_REPO}/.ci/oci-common.sh"

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTests() {
  . "${OPERATOR_REPO}"/olm/testUpdate.sh -c stable -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -n ${NAMESPACE}
}

initDefaults
runTests
