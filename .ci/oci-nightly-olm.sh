#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

################################ !!!   IMPORTANT   !!! ################################
########### THIS JOB use Openshift CI operators workflows to run nightly olm tests ####
##########  More info about how it is configured can be found here: https://docs.ci.openshift.org/docs/how-tos/testing-operator-sdk-operators #############
#######################################################################################################################################################

export XDG_CONFIG_HOME=/tmp/chectl/config
export XDG_CACHE_HOME=/tmp/chectl/cache
export XDG_DATA_HOME=/tmp/chectl/data


# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Detect the base directory where che-operator is cloned
SCRIPT=$(readlink -f "$0")
export SCRIPT

OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")");
export OPERATOR_REPO

# Artifacts dir where job will store all che events and logs
ARTIFACTS_DIR="/tmp/artifacts"
export ARTIFACTS_DIR

# Execute olm nightly files in openshift
PLATFORM="openshift"
export PLATFORM

# Test nightly olm files
NAMESPACE="eclipse-che"
export NAMESPACE

# CI_CHE_OPERATOR_IMAGE it is che operator image builded in openshift CI job workflow. More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
export OPERATOR_IMAGE=${CI_CHE_OPERATOR_IMAGE:-"quay.io/eclipse/che-operator:nightly"}

# Get nightly CSV
export CSV_FILE
CSV_FILE="${OPERATOR_REPO}/deploy/olm-catalog/eclipse-che-preview-${PLATFORM}/manifests/che-operator.clusterserviceversion.yaml"

# Import common functions utilities
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Patch che operator image with image builded from source in Openshift CI job.
function patchCheOperatorImage() {
    echo "[INFO] Getting che operator pod name..."
    OPERATOR_POD=$(oc get pods -o json -n ${NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name')
    oc patch pod ${OPERATOR_POD} -n ${NAMESPACE} --type='json' -p='[{"op": "replace", "path": "/spec/containers/0/image", "value":'${OPERATOR_IMAGE}'}]'
    
    # The following command retrieve the operator image
    OPERATOR_POD_IMAGE=$(oc get pods -n ${NAMESPACE} -o json | jq -r '.items[] | select(.metadata.name | test("che-operator-")).spec.containers[].image')
    echo "[INFO] CHE operator image is ${OPERATOR_POD_IMAGE}"
}

# Create CheCluster object in Openshift ci with desired values
function applyCRCheCluster() {
  echo "Creating Custom Resource"
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${CSV_FILE}")
  CR=$(echo "$CRs" | yq -r ".[0]")
  if [ "${PLATFORM}" == "openshift" ] && [ "${OAUTH}" == "false" ]; then
    CR=$(echo "$CR" | yq -r ".spec.auth.openShiftoAuth = false")
  fi
  if [ "${CHE_EXPOSURE_STRATEGY}" == "single-host" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.server.serverExposureStrategy = \"${CHE_EXPOSURE_STRATEGY}\"")
  fi
  echo -e "$CR"
  echo "$CR" | oc apply -n "${NAMESPACE}" -f -
}

# Run che deployment after patch operator image.
function deployEclipseChe() {
    export OAUTH="false"

    # Deploy Eclipse Che applying CR
    applyCRCheCluster
    waitEclipseCheDeployed

    # Create a workspace
    getCheAcessToken
    chectl workspace:create --start --chenamespace=${NAMESPACE} --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

    # Start a workspace and wait until workspace it is alive
    getCheAcessToken
    chectl workspace:list --chenamespace=${NAMESPACE}
    waitWorkspaceStart
}

printOlmCheObjects
patchCheOperatorImage
deployEclipseChe
