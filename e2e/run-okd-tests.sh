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

init() {
  OC_VERSION="v3.11.0-0cbc58b"
  GO_TOOLSET_VERSION="1.11.5-3"
  IP_ADDRESS="172.17.0.1"
  SCRIPT=$(readlink -f "$0") # this script's absolute path
  SCRIPTPATH=$(dirname "$SCRIPT") # /path/to/e2e/ folder
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then OPERATOR_REPO=${WORKSPACE}; else OPERATOR_REPO=$(dirname "$SCRIPTPATH"); fi
}

oc_installation() {
  if [ ! -f "$OPERATOR_REPO/tmp/oc" ]; then
    if [ ! -d "$OPERATOR_REPO/tmp" ]; then mkdir -p "$OPERATOR_REPO/tmp" && chmod 775 "$OPERATOR_REPO/tmp"; fi

    echo "[INFO] Downloading Openshift3.11 binaries..."
    curl -s -S -L https://github.com/openshift/origin/releases/download/${OC_VERSION%%-*}/openshift-origin-client-tools-${OC_VERSION}-linux-64bit.tar.gz \
      -o ${OPERATOR_REPO}/tmp/oc.tar && tar -xvf ${OPERATOR_REPO}/tmp/oc.tar -C ${OPERATOR_REPO}/tmp --strip-components=1
  fi
  echo "[INFO] Sarting a new OC cluster."
  cd "$OPERATOR_REPO/tmp"
  ./oc cluster up --public-hostname=${IP_ADDRESS} --routing-suffix=${IP_ADDRESS}.nip.io
  ./oc login -u system:admin && ./oc adm policy add-cluster-role-to-user cluster-admin developer && ./oc login -u developer -p developer
}

oc_tls_mode() {
    # generate self signed cert
    echo "[INFO] Generate self signed certificate"
    openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -subj "/CN=*.${IP_ADDRESS}.nip.io" -nodes && cat cert.pem key.pem > ca.crt
    # replace default router cert
    echo "[INFO] Update OpenShift router tls secret"
    ./oc project default
    ./oc secrets new router-certs tls.crt=ca.crt tls.key=key.pem -o json --type='kubernetes.io/tls' --confirm | ./oc replace -f -
    echo "[INFO] Initiate a new router deployment"
    sleep 20
    ./oc rollout latest dc/router -n=default || true
}

run_tests() {
  oc_installation
  oc_tls_mode
  echo "[INFO] Compile tests binary"
  docker run -t \
              -v ${OPERATOR_REPO}/tmp:/operator \
              -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.access.redhat.com/devtools/go-toolset-rhel7:${GO_TOOLSET_VERSION} \
              sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /operator/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"
  echo "[INFO] Run tests..."
  ./run-tests
}

init
run_tests
