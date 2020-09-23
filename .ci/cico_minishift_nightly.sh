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

  if [ "$result" != "0" ]; then
    echo "[ERROR] Please check the artifacts in github actions"
    getOCCheClusterLogs
    exit 1
  fi

  echo "[INFO] Job finished Successfully.Please check the artifacts in github actions"
  getOCCheClusterLogs

  exit $result
}

# Define global environments
function init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export NAMESPACE="che"
  export PLATFORM="openshift"

  # Set operator root directory
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi
}

# Utility to get che events and pod logs from openshift cluster
function getOCCheClusterLogs() {
  mkdir -p /tmp/artifacts-che
  cd /tmp/artifacts-che

  for POD in $(oc get pods -o name -n ${NAMESPACE}); do
    for CONTAINER in $(oc get -n ${NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
      echo ""
      echo "[INFO] Getting logs from $POD"
      echo ""
      oc logs ${POD} -c ${CONTAINER} -n ${NAMESPACE} |tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
    done
  done
  echo "[INFO] Get events"
  oc get events -n ${NAMESPACE}| tee get_events.log
  oc get all | tee get_all.log
}

# Deploy Eclipse Che
function run() {
    cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  auth:
    updateAdminPassword: false
    openShiftoAuth: false
EOL
    echo "======= Che cr patch ======="
    cat /tmp/che-cr-patch.yaml

    # OPERATOR_IMAGE In CI is defined in .github/workflows/che-nightly.yaml
    export OPERATOR_IMAGE="quay.io/eclipse/che-operator:test"

    cat deploy/operator.yaml | \
      sed 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|' | \
      sed 's|quay.io/eclipse/che-operator:nightly|'${OPERATOR_IMAGE}'|' | \
      oc apply -n ${NAMESPACE} -f -

    chectl server:start --platform=minishift --skip-kubernetes-health-check --installer=operator --chenamespace=${NAMESPACE} --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml --che-operator-image ${OPERATOR_IMAGE}

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
