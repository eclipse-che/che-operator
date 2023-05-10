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

source "${OPERATOR_REPO}/build/scripts/minikube-tests/common.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  chectl server:deploy \
    --batch \
    --platform minikube  \
    --templates ${PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH} \
    --k8spodwaittimeout=120000 \
    --k8spodreadytimeout=120000 \
    --che-operator-cr-patch-yaml "${OPERATOR_REPO}/build/scripts/minikube-tests/minikube-checluster-patch.yaml"

  createDevWorkspace
  startAndWaitDevWorkspace
  stopAndWaitDevWorkspace

  # Free up some cpu resources
  kubectl scale deployment che --replicas=0 -n eclipse-che

  chectl server:update --templates ${LAST_OPERATOR_VERSION_TEMPLATE_PATH} --batch

  # Wait until Eclipse Che is deployed
  pushd ${OPERATOR_REPO}
    make wait-devworkspace-running NAMESPACE="devworkspace-controller"
    make wait-eclipseche-version VERSION="${LAST_PACKAGE_VERSION}" NAMESPACE=${NAMESPACE}
  popd

  startAndWaitDevWorkspace
  stopAndWaitDevWorkspace
  deleteDevWorkspace
}

initDefaults
initTemplates
runTest
