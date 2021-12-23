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
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  chectl server:deploy \
    --batch \
    --platform minikube \
    --installer operator \
    --version ${PREVIOUS_PACKAGE_VERSION}

  createWorkspace

  chectl server:update --batch --templates=$LAST_OPERATOR_TEMPLATE
  waitEclipseCheDeployed ${LAST_PACKAGE_VERSION}

  startExistedWorkspace
  waitWorkspaceStart
}

initDefaults
initStableTemplates "kubernetes" "stable"
runTest
