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

export OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

unset OPERATOR_TEST_NAMESPACE

# Discover test namespace
# Eclipse Che subscription is pre-created by OpenShift CI
discoverOperatorTestNamespace() {
  discoverEclipseCheSubscription
  OPERATOR_TEST_NAMESPACE=${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}
}

# Delete Eclipse Che next version operator by deleting its subscription
deleteEclipseCheNextVersionSubscription() {
  discoverEclipseCheSubscription

  # save .spec to recreate subscription later
  ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${OPERATOR_TEST_NAMESPACE} -o "jsonpath={.spec.source}")
  ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE_NAMESPACE=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${OPERATOR_TEST_NAMESPACE} -o "jsonpath={.spec.sourceNamespace}")

  oc delete csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${OPERATOR_TEST_NAMESPACE}
  oc delete subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${OPERATOR_TEST_NAMESPACE}
}

# Install Eclipse Che next version operator by creating its subscription
createEclipseCheNextVersionSubscription() {
  pushd "${OPERATOR_REPO}" || exit 1

  make create-subscription \
    NAME=${ECLIPSE_CHE_SUBSCRIPTION_NAME} \
    NAMESPACE=${OPERATOR_TEST_NAMESPACE} \
    SOURCE=${ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE} \
    SOURCE_NAMESPACE=${ECLIPSE_CHE_NEXT_SUBSCRIPTION_SPEC_SOURCE_NAMESPACE} \
    PACKAGE_NAME=${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME} \
    CHANNEL="next" \
    INSTALL_PLAN_APPROVAL="Auto"

  popd
}

# Install Eclipse Che stable version operator by creating its subscription
createEclipseCheStableVersionSubscription() {
  pushd "${OPERATOR_REPO}" || exit 1

  make create-subscription \
    NAME=${ECLIPSE_CHE_SUBSCRIPTION_NAME} \
    NAMESPACE=${OPERATOR_TEST_NAMESPACE} \
    SOURCE="community-operators" \
    SOURCE_NAMESPACE="openshift-marketplace" \
    PACKAGE_NAME=${ECLIPSE_CHE_STABLE_PACKAGE_NAME} \
    CHANNEL="stable" \
    INSTALL_PLAN_APPROVAL="Auto"

  popd
}

# Uninstall Eclipse Che stable version operator by deleting its subscription
deleteEclipseCheStableVersionSubscription() {
  discoverEclipseCheSubscription

  oc delete csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${OPERATOR_TEST_NAMESPACE}
  oc delete subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${OPERATOR_TEST_NAMESPACE}
}

runTests() {
  discoverOperatorTestNamespace

  # Uninstall pre-created Eclipse Che next version operator (operands don't exist)
  deleteEclipseCheNextVersionSubscription
  waitForRemovedEclipseCheSubscription

  # Deploy stable version
  createEclipseCheStableVersionSubscription
  waitForInstalledEclipseCheCSV
  getCheClusterCRFromInstalledCSV | oc apply -n "${NAMESPACE}" -f -

  pushd ${OPERATOR_REPO}
    make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE=${NAMESPACE}
  popd

  # Delete Eclipse Che stable version (just operator)
  deleteEclipseCheStableVersionSubscription
  waitForRemovedEclipseCheSubscription
  # Hack, since we remove operator pod, webhook won't work.
  # We have to disable it for a while.
  oc patch crd checlusters.org.eclipse.che --patch '{"spec": {"conversion": null}}' --type=merge

  # Install Eclipse Che next version
  createEclipseCheNextVersionSubscription
  waitForInstalledEclipseCheCSV
  # CI_CHE_OPERATOR_IMAGE it is che operator image built in openshift CI job workflow.
  # More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  useCustomOperatorImageInCSV "${CI_CHE_OPERATOR_IMAGE}"
  pushd ${OPERATOR_REPO}
    make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE=${NAMESPACE}
  popd
}

runTests
