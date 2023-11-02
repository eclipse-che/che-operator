#!/usr/bin/env bash
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
  buildAndCopyCheOperatorImageToMinikube
  yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml"
  yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml"

  chectl server:deploy \
    --batch \
    --platform minikube \
    --k8spodwaittimeout=6000000 \
    --k8spodreadytimeout=6000000 \
    --templates "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}" \
    --che-operator-cr-patch-yaml "${OPERATOR_REPO}/build/scripts/minikube-tests/minikube-checluster-patch.yaml"

  make wait-devworkspace-running NAMESPACE="devworkspace-controller" VERBOSE=1

  # Free up some cpu resources
  kubectl scale deployment che --replicas=0 -n eclipse-che

  createDevWorkspace
  startAndWaitDevWorkspace
  stopAndWaitDevWorkspace
  deleteDevWorkspace
}

pushd ${OPERATOR_REPO} >/dev/null
initDefaults
initTemplates
runTest
popd >/dev/null
