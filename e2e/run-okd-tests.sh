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

# Exit on error
set -e -x

trap 'Catch_Finish $?' EXIT SIGINT

# Catch errors and force to delete minishift VM.
cleanup() {
  echo "[INFO] Deleting minishift VM..."
  yes | ./tmp/minishift delete && rm -rf ~/.minishift ${OPERATOR_REPO}/tmp

}

Catch_Finish() {
  if [ $1 != 0 ]; then
    echo "[ERROR] Please check the output error"
    cleanup
  else
    echo "[INFO] Script executed successfully: $0!"
    cleanup
  fi
}

init() {
  MSFT_RELEASE="1.34.2"
  GO_TOOLSET_VERSION="1.11.5-3"
  IP_ADDRESS="172.17.0.1"
  SCRIPT=$(readlink -f "$0") # this script's absolute path
  SCRIPTPATH=$(dirname "$SCRIPT") # /path/to/e2e/ folder
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then OPERATOR_REPO=${WORKSPACE}; else OPERATOR_REPO=$(dirname "$SCRIPTPATH"); fi
}

minishift_installation() {
  if [ ! -f "$OPERATOR_REPO/tmp/minishift" ]; then
    if [ ! -d "$OPERATOR_REPO/tmp" ]; then mkdir -p "$OPERATOR_REPO/tmp" && chmod 777 "$OPERATOR_REPO/tmp"; fi
    echo "[INFO] Downloading Minishift binaries..."
    curl -s -S -L https://github.com/minishift/minishift/releases/download/v$MSFT_RELEASE/minishift-$MSFT_RELEASE-linux-amd64.tgz \
      -o ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar && tar -xvf ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar -C ${OPERATOR_REPO}/tmp --strip-components=1
  fi
  cd "$OPERATOR_REPO/tmp"
  echo "[INFO] Sarting a new OC cluster."
  ./minishift start --memory=4096 && eval $(./minishift oc-env)
  oc login -u system:admin
  oc adm policy add-cluster-role-to-user cluster-admin developer && oc login -u developer -p developer
}

oc_tls_mode() {
    # generate self signed cert
    echo "[INFO] Generate self signed certificate"
    openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -subj "/CN=*.${IP_ADDRESS}.nip.io" -nodes && cat cert.pem key.pem > ca.crt
    # replace default router cert
    echo "[INFO] Update OpenShift router tls secret"
    oc project default
    oc secrets new router-certs tls.crt=ca.crt tls.key=key.pem -o json --type='kubernetes.io/tls' --confirm | oc replace -f -
    echo "[INFO] Initiate a new router deployment"
    sleep 20
    oc rollout latest dc/router -n=default || true
}

run_tests() {
  minishift_installation
  
  echo "[INFO] Register a custom resource definition"
  oc apply -f ${OPERATOR_REPO}/deploy/crds/org_v1_che_crd.yaml

  oc_tls_mode
  
  echo "[INFO] Compile tests binary"
  docker run -t \
              -v ${OPERATOR_REPO}/tmp:/operator \
              -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.access.redhat.com/devtools/go-toolset-rhel7:${GO_TOOLSET_VERSION} \
              sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /operator/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"
  
  echo "[INFO] Build operator docker image and load in to minishift VM..."
  cd ${OPERATOR_REPO} && docker build -t che/operator -f Dockerfile . && docker save che/operator > operator.tar
  eval $(./tmp/minishift docker-env) && docker load -i operator.tar && rm operator.tar
  
  echo "[INFO] Run tests..."
  ./tmp/run-tests
}

init

source ./cico_common.sh
installStartDocker
install_required_packages
start_libvirt
setup_kvm_machine_driver

run_tests
