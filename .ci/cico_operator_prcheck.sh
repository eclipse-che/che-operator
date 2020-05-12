#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

trap 'Catch_Finish $?' EXIT SIGINT

# Catch errors and force to delete minishift VM.
Catch_Finish() {
  rm -rf ${OPERATOR_REPO}/tmp ~/.minishift && yes | minikube delete
}

init() {
  GO_TOOLSET_VERSION="1.12.12-4"
  SCRIPT=$(readlink -f "$0") # this script's absolute path
  SCRIPTPATH=$(dirname "$SCRIPT") # /path/to/e2e/ folder
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then OPERATOR_REPO=${WORKSPACE}; else OPERATOR_REPO=$(dirname "$SCRIPTPATH"); fi
}

oc_tls_mode() {
    # generate self signed cert
    printInfo "Generate self signed certificate"
    cd "$OPERATOR_REPO/tmp"
    generate_self_signed_certs
    # replace default router cert
    printInfo "Update OpenShift router tls secret"
    oc project default
    oc secrets new router-certs tls.crt=ca.crt tls.key=key.pem -o json --type='kubernetes.io/tls' --confirm | oc replace -f -
    printInfo "Initiate a new router deployment"
    sleep 20
    oc rollout latest dc/router -n=default || true
}

run_tests() {
  # Download minikube binary
  curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl
  chmod +x ./kubectl
  sudo mv ./kubectl /usr/local/bin/kubectl

  echo $CRW_BOTS_PULL_SECRETS >> pull-secrets.txt
  yum install --assumeyes NetworkManager
  source ${OPERATOR_REPO}/.ci/start-crc.sh

  eval $( crc oc-env )
  oc login -u kubeadmin -p $(cat ~/.crc/cache/*/kubeadmin-password) https://api.crc.testing:6443 --insecure-skip-tls-verify

  ${OPERATOR_REPO}/olm/testUpdate.sh openshift stable che

  yes | crc delete
  rm -rf  ~/.crc && rm -rf ~/.kube

  source ${OPERATOR_REPO}/.ci/start-minikube.sh
  ${OPERATOR_REPO}/olm/testUpdate.sh kubernetes stable che

}

init

source ${OPERATOR_REPO}/.ci/util/ci_common.sh
installJQ
installYQ
load_jenkins_vars
installStartDocker
install_VirtPackages
start_libvirt
setup_kvm_machine_driver
run_tests
