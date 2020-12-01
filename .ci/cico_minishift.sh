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

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Catch_Finish is executed after finish script.
catchFinish() {
  result=$?

  collectCheLogWithChectl
  if [ "$result" != "0" ]; then
    echo "[ERROR] Job failed."
  else
    echo "[INFO] Job completed successfully."
  fi

  echo "[INFO] Please check github actions artifacts."
  exit $result
}

# Define global environments
function init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export NAMESPACE="che"
  export PLATFORM="openshift"
  export INSTALLER="operator"

  # OPERATOR_IMAGE In CI is defined in .github/workflows/che-nightly.yaml
  export OPERATOR_IMAGE="quay.io/eclipse/che-operator:test"

  # Set operator root directory
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi
}

function prepareTemplates() {
  rm -rf ${OPERATOR_REPO}/tmp

  # prepare template folder
  mkdir -p "${OPERATOR_REPO}/tmp/che-operator" && chmod 777 "${OPERATOR_REPO}/tmp"
  cp -rf ${OPERATOR_REPO}/deploy/* "${OPERATOR_REPO}/tmp/che-operator"

  # prepare CR
  yq -riSY  '.spec.auth.updateAdminPassword = false' "${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml"
  yq -riSY  '.spec.auth.openShiftoAuth = false' "${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml"

  # update operator yaml
  yq -riSY  '.spec.template.spec.containers[0].image = '${OPERATOR_IMAGE} "${OPERATOR_REPO}/tmp/che-operator/operator.yaml"
  yq -riSY  '.spec.template.spec.containers[0].imagePullPolicy = IfNotPresent' "${OPERATOR_REPO}/tmp/che-operator/operator.yaml"

  cat ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml
  cat ${OPERATOR_REPO}/tmp/che-operator/operator.yaml
}

function deployEclipseChe() {
  # Deploy Eclipse Che
  chectl server:deploy --platform=${PLATFORM} \
    --installer ${INSTALLER} \
    --chenamespace ${NAMESPACE} \
    --che-operator-image ${OPERATOR_IMAGE} \
    --che-operator-cr-yaml ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml \
    --templates ${OPERATOR_REPO}/tmp
}

function startWorkspace() {
  # Create and start a workspace
  chectl auth:login -u admin -p admin
  chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

  # Wait for workspace to be up
  waitWorkspaceStart
}

function runTest() {
  deployEclipseChe
  startWorkspace
}

source "${OPERATOR_REPO}"/.ci/util/ci_common.sh

init
installYq
prepareTemplates
runTest
