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

# Get absolute path for root repo directory from github actions context: https://docs.github.com/en/free-pro-team@latest/actions/reference/context-and-expression-syntax-for-github-actions
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$SCRIPT")")")")
fi
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}/olm/olm.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

patchTemplates() {
  disableUpdateAdminPassword ${LAST_OPERATOR_TEMPLATE}
  setIngressDomain ${LAST_OPERATOR_TEMPLATE} "$(minikube ip).nip.io"
  setCustomOperatorImage ${TEMPLATES} ${OPERATOR_IMAGE}
}

runTest() {
  deployEclipseCheWithTemplates "operator" "minikube" "quay.io/eclipse/che-operator:${LAST_PACKAGE_VERSION}" ${LAST_OPERATOR_TEMPLATE}
  createWorkspace

  updateEclipseChe ${OPERATOR_IMAGE} ${TEMPLATES}
  waitEclipseCheDeployed "next"

  startExistedWorkspace
  waitWorkspaceStart
}

initDefaults
installOperatorMarketPlace
initLatestTemplates
initStableTemplates "kubernetes" "stable"
patchTemplates
buildCheOperatorImage
copyCheOperatorImageToMinikube
runTest
