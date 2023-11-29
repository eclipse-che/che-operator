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

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/minikube-tests/common.sh"

init() {
  unset CR_PATCH_YAML

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--help'|'-h') usage; exit;;
      '--cr-patch-yaml') CR_PATCH_YAML=$2; shift 1;;
    esac
    shift 1
  done
}

usage () {
  echo "Deploy Eclipse Che from sources"
  echo
	echo "Usage:"
	echo -e "\t$0 [--cr-patch-yaml <path_to_cr_patch>]"
  echo
  echo "OPTIONS:"
  echo -e "\t--cr-patch-yaml          CheCluster CR patch yaml file"
  echo
	echo "Example:"
	echo -e "\t$0"
}

runTest() {
  buildAndCopyCheOperatorImageToMinikube
  yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml"
  yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml"

  if [[ -n "${CR_PATCH_YAML}" ]]; then
      chectl server:deploy --batch --platform minikube \
        --templates "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}" \
        --che-operator-cr-patch-yaml "${CR_PATCH_YAML}"
  else
    chectl server:deploy --batch --platform minikube \
      --templates "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}"
  fi
}

pushd ${OPERATOR_REPO} >/dev/null
initDefaults
init "$@"
initTemplates
runTest
popd >/dev/null
