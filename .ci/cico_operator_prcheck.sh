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

set -e -x

trap 'Catch_Finish $?' EXIT SIGINT

# Catch errors and force to delete minishift VM.
Catch_Finish() {
  rm -rf ${OPERATOR_REPO}/tmp ~/.minishift && yes | minishift delete
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
  printInfo "Register a custom resource definition"
  oc apply -f ${OPERATOR_REPO}/deploy/crds/org_v1_che_crd.yaml

  oc_tls_mode
    
  printInfo "Starting to compile e2e tests binary"
  docker run -t \
              -v ${OPERATOR_REPO}/tmp:/operator \
              -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.access.redhat.com/devtools/go-toolset-rhel7:${GO_TOOLSET_VERSION} \
              sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /operator/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"
  
  printInfo "Build operator docker image and load in to minishift VM..."

  OPERATOR_IMAGE="quay.io/eclipse/che-operator:nightly"
  cd "$OPERATOR_REPO" && docker build -t "${OPERATOR_IMAGE}" -f Dockerfile . && docker save "${OPERATOR_IMAGE}" > operator.tar
  eval $(minishift docker-env) && docker load -i operator.tar && rm operator.tar
  
  printInfo "Runing e2e tests..."
  ${OPERATOR_REPO}/tmp/run-tests
}

init

source ${OPERATOR_REPO}/.ci/util/ci_common.sh
installJQ
load_jenkins_vars
installStartDocker
install_VirtPackages
start_libvirt
setup_kvm_machine_driver
minishift_installation
run_tests
