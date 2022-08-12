#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

################################ !!!   IMPORTANT   !!! ################################
########### THIS JOB USE openshift ci operators workflows to run  #####################
##########  More info about how it is configured can be found here: https://docs.ci.openshift.org/docs/how-tos/testing-operator-sdk-operators #############
#######################################################################################################################################################

set -ex

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}/.github/bin/common.sh"
source "${OPERATOR_REPO}/.ci/oci-common.sh"

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Uninstall Eclipse Che next version operator by removing subscription
deleteEclipseCheNextSubscription() {
  findEclipseCheSubscription

  # save .spec to recreate subscription later
  ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o "jsonpath={.spec.source}")
  ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE_NAMESPACE=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o "jsonpath={.spec.sourceNamespace}")

  oc delete csv ${ECLIPSE_CHE_NEXT_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}
  oc delete subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}
}

# Install Eclipse Che next version operator by recreating subscription
createEclipseCheNextSubscription() {
  pushd "${OPERATOR_REPO}" || exit 1

  make create-subscription \
    NAME=${ECLIPSE_CHE_SUBSCRIPTION_NAME} \
    SOURCE=${ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE} \
    SOURCE_NAMESPACE=${ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE_NAMESPACE} \
    PACKAGE_NAME=${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME} \
    CHANNEL="next" \
    INSTALL_PLAN_APPROVAL="Auto"

  popd
}

# Uninstall Eclipse Che stable version operator by removing subscription
deleteEclipseCheStableSubscription() {
  findEclipseCheSubscription

  oc delete csv ${ECLIPSE_CHE_NEXT_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}
  oc delete subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}
}

runTests() {
  deleteEclipseCheNextSubscription

  # Deploy stable version
  chectl server:deploy --platform openshift --olm-channel stable

  # Delete Eclipse Che stable version operator
  deleteEclipseCheStableSubscription

  # Install Eclipse Che next version operator
  createEclipseCheNextSubscription

  # CI_CHE_OPERATOR_IMAGE it is che operator image built in openshift CI job workflow.
  # More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  useCustomOperatorImageInCSV "${CI_CHE_OPERATOR_IMAGE}"
  waitEclipseCheDeployed "next"
}

initDefaults
runTests
