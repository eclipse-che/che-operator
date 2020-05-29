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
  printInfo "http://artifacts.ci.centos.org/devtools/che/che-operator-olm-pr-check/report/"
  archiveArtifacts "che-operator-olm-pr-check"
}

init() {
  SCRIPT=$(readlink -f "$0")
  SCRIPTPATH=$(dirname "$SCRIPT")
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then OPERATOR_REPO=${WORKSPACE}; else OPERATOR_REPO=$(dirname "$SCRIPTPATH"); fi
  RAM_MEMORY=8192
  NAMESPACE="che-default"
  CHANNEL="nightly"
}

minishift_olm_installation() {
  MSFT_RELEASE="1.34.2"
  printInfo "Downloading Minishift binaries"
  if [ ! -d "$OPERATOR_REPO/tmp" ]; then mkdir -p "$OPERATOR_REPO/tmp" && chmod 777 "$OPERATOR_REPO/tmp"; fi
  curl -L https://github.com/minishift/minishift/releases/download/v$MSFT_RELEASE/minishift-$MSFT_RELEASE-linux-amd64.tgz \
    -o ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar && tar -xvf ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar -C /usr/bin --strip-components=1
}

install_Dependencies() {
  installYQ
  installJQ
  install_VirtPackages
  installStartDocker
}

run_olm_tests() {
  for platform in 'openshift' 'kubernetes'
  do
    if [[ ${platform} == 'kubernetes' ]]; then
      printInfo "Starting minikube VM to test kubernetes olm files..."
      source ${OPERATOR_REPO}/.ci/start-minikube.sh
  
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
