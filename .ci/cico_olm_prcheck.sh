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
  RAM_MEMORY=8192
  NAMESPACE="che-default"
  CHANNEL="stable"
}

install_Dependencies() {
  installYQ
  install_VirtPackages
  installStartDocker
  minikube_installation
}

run_olm_tests() {
  for platform in 'openshift' 'kubernetes'
  do
    if [[ ${platform} == 'openshift' ]]; then
      printInfo "Starting minishift VM to test openshift olm files..."
      minishift start --memory=${RAM_MEMORY}
      oc login -u system:admin
      oc adm policy add-cluster-role-to-user cluster-admin developer && oc login -u developer -p developer

      sh "${OPERATOR_REPO}"/olm/testCatalogSource.sh ${platform} ${CHANNEL} ${NAMESPACE}
      printInfo "Successfully verified olm files on openshift platform."
      rm -rf ~/.kube .minishift && yes | minishift delete --force --clear-cache
    fi
    if [[ ${platform} == 'kubernetes' ]]; then
      printInfo "Starting minikube VM to test kubernetes olm files..."
      minikube start --memory=${RAM_MEMORY}
      sh "${OPERATOR_REPO}"/olm/testCatalogSource.sh ${platform} ${CHANNEL} ${NAMESPACE}
      printInfo "Successfully verified olm files on kubernetes platform."
      rm -rf ~/.kube && yes | minikube delete
    fi
  done
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
install_Dependencies
run_olm_tests
