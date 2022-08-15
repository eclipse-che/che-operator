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
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
fi

source "${OPERATOR_REPO}/build/scripts/common.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  deployEclipseCheWithOperator "/tmp/chectl-${LAST_PACKAGE_VERSION}/chectl/bin/run" "minikube" "${LAST_OPERATOR_VERSION_TEMPLATE_PATH}" "false"
  updateEclipseChe "chectl" "minikube" "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}" "true"
}

initDefaults
initTemplates
installchectl "${LAST_PACKAGE_VERSION}"
runTest
