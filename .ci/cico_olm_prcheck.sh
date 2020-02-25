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

trap 'Catch_Finish $?' EXIT SIGINT

# Catch errors and force to delete minikube VM.
Catch_Finish() {
  rm -rf ~/.kube && yes | minikube delete
}

init() {
  SCRIPT=$(readlink -f "$0")
  SCRIPTPATH=$(dirname "$SCRIPT")
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then OPERATOR_REPO=${WORKSPACE}; else OPERATOR_REPO=$(dirname "$SCRIPTPATH"); fi
  MINIKUBE_MEMORY=8192
  NAMESPACE="che-default"
  CHANNEL="nightly"
}

install_Dependencies() {
  installYQ
  install_VirtPackages
  installStartDocker
  minikube_installation
}

run_olm_tests() {
  printInfo "Starting minikube VM to test kubernetes olm files..."
  minikube start --memory=${MINIKUBE_MEMORY}
  printInfo "Running olm files tests on Kubernetes"
  sh ${OPERATOR_REPO}/olm/testCatalogSource.sh kubernetes ${CHANNEL} ${NAMESPACE}
  printInfo "Successfully verified olm files on kubernetes platform."
}

init
source ${OPERATOR_REPO}/.ci/util/ci_common.sh
install_Dependencies
run_olm_tests
