#!/usr/bin/env bash
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

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u
# print each command before executing it
set -x

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Define global environments
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
export RAM_MEMORY=8192
export NAMESPACE="che"
export PLATFORM="kubernetes"

# Directory where che artifacts will be stored and uploaded to GH actions artifacts
export ARTIFACTS_DIR="/tmp/artifacts-che"

# Set operator root directory
export OPERATOR_IMAGE="che-operator:pr-check"

# Catch_Finish is executed after finish script.
catchFinish() {
  result=$?

  if [ "$result" != "0" ]; then
    echo "[ERROR] Please check the github actions artifacts"
    collectCheLogWithChectl
    exit 1
  fi

  echo "[INFO] Job finished Successfully.Please check github actions artifacts"
  collectCheLogWithChectl

  exit $result
}

# Utility to get che events and pod logs from openshift cluster
function collectCheLogWithChectl() {
  mkdir -p ${ARTIFACTS_DIR}
  chectl server:logs --directory=${ARTIFACTS_DIR}
}

# Deploy Eclipse Che in single host mode(gateway exposure type)
function runSHostGatewayExposure() {
    # Patch file to pass to chectl
    cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  server:
    serverExposureStrategy: 'single-host'
  auth:
    updateAdminPassword: false
    openShiftoAuth: false
  k8s:
    singleHostExposureType: 'gateway'
EOL
    echo "======= Che cr patch ======="
    cat /tmp/che-cr-patch.yaml

    # Use custom changes, don't pull image from quay.io
    kubectl create namespace che
    cat ${OPERATOR_REPO}/deploy/operator.yaml | \
    sed 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|' | \
    sed 's|quay.io/eclipse/che-operator:nightly|'${OPERATOR_IMAGE}'|' | \
    oc apply -n ${NAMESPACE} -f -

    # Start to deploy Che
    chectl server:start --platform=minikube --skip-kubernetes-health-check --installer=operator \
        --chenamespace=${NAMESPACE} --che-operator-image=${OPERATOR_IMAGE} --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml

    # Wait for workspace to be up for native deployment from ${PROJECT_DIR}/.github/action_scripts/minikube/function-utilities.sh
    getSingleHostToken
    chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    # Wait for workspace to be up
    waitSingleHostWorkspaceStart
}

source "${OPERATOR_REPO}"/.github/action_scripts/nightly/minikube/function-utilities.sh
echo "[INFO] Start to Building Che Operator Image"
buildCheOperatorImage

echo "[INFO] Start to run single host with gateway exposure mode"
runSHostGatewayExposure
