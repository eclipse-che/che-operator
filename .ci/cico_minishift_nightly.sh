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

set -ex

init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export NAMESPACE="che"
  export PLATFORM="openshift"

  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

}

installDependencies() {
  install_VirtPackages
  installStartDocker
  start_libvirt
  setup_kvm_machine_driver
  minishift_installation
  installChectl
  load_jenkins_vars
}

self_signed_minishift() {
  export DOMAIN=*.$(minishift ip).nip.io

  source ${OPERATOR_REPO}/.ci/util/che-cert-generation.sh

  #Configure Router with generated certificate:
  oc login -u system:admin --insecure-skip-tls-verify=true
  oc project default
  oc delete secret router-certs

  cat domain.crt domain.key > minishift.crt
  oc create secret tls router-certs --key=domain.key --cert=minishift.crt
  oc rollout latest router

  oc create namespace che

  cp rootCA.crt ca.crt
  oc create secret generic self-signed-certificate --from-file=ca.crt -n=che
}

run() {
    cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  auth:
    updateAdminPassword: false
EOL

    self_signed_minishift
    
    echo "======= Che cr patch ======="
    cat /tmp/che-cr-patch.yaml  
    chectl server:start --platform=minishift --skip-kubernetes-health-check --installer=operator --chenamespace=${NAMESPACE} --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml
    
    # Create and start a workspace
    getCheAcessToken # Function from ./util/ci_common.sh
    chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    # Wait for workspace to be up
    waitWorkspaceStart  # Function from ./util/ci_common.sh
    oc get events -n ${NAMESPACE}
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
run
