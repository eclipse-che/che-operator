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

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}/.github/bin/common.sh"

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

overrideDefaults() {
  OPERATOR_IMAGE=${CI_CHE_OPERATOR_IMAGE}
}

runTests() {
  deployEclipseCheWithOperator "chectl" "openshift" ${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH} "true"
}

initDefaults
initTemplates
overrideDefaults

runTests
