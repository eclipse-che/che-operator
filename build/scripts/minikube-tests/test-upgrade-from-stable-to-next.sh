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
  # Deploy stable version
  chectl server:deploy \
    --batch \
    --platform minikube \
    --k8spodwaittimeout=6000000 \
    --k8spodreadytimeout=6000000 \
    --templates ${LAST_OPERATOR_VERSION_TEMPLATE_PATH} \
    --che-operator-cr-patch-yaml "${OPERATOR_REPO}/build/scripts/minikube-tests/minikube-checluster-patch.yaml"

  createDevWorkspace

  # Free up some cpu resources
  kubectl scale deployment che --replicas=0 -n eclipse-che

  # Update to next version
  buildAndCopyCheOperatorImageToMinikube
  yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml
  yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "Never"' ${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml
  chectl server:update --batch --templates ${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}

  # Wait until Eclipse Che is deployed
  pushd ${OPERATOR_REPO}
    make wait-devworkspace-running NAMESPACE="devworkspace-controller"
    make wait-eclipseche-version VERSION="next" NAMESPACE=${NAMESPACE}
  popd

  # Free up some resources
  minikube image rm quay.io/eclipse/che-plugin-registry:${LAST_PACKAGE_VERSION}
  minikube image rm quay.io/eclipse/che-devfile-registry:${LAST_PACKAGE_VERSION}
  minikube image rm quay.io/eclipse/che-dashboard:${LAST_PACKAGE_VERSION}
  minikube image rm quay.io/eclipse/che-server:${LAST_PACKAGE_VERSION}
  minikube image rm quay.io/eclipse/che-operator:${LAST_PACKAGE_VERSION}

  # Free up some cpu resources
  kubectl scale deployment che --replicas=0 -n eclipse-che

  startAndWaitDevWorkspace
  stopAndWaitDevWorkspace
  deleteDevWorkspace
}

initDefaults
initTemplates
runTest

