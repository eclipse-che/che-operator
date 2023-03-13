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

# Uninstall Eclipse Che stable version operator by deleting its subscription
deleteEclipseCheStableVersionOperator() {
  discoverEclipseCheSubscription

  oc delete csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}
  oc delete subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE}

  waitForRemovedSubscription ${ECLIPSE_CHE_PACKAGE_NAME}

  # Hack, since we remove operator pod, webhook won't work.
  # We have to disable it for a while.
  oc patch crd checlusters.org.eclipse.che --patch '{"spec": {"conversion": null}}' --type=merge
}

deleteDevWorkspaceStableVersionOperator() {
  DEV_WORKSPACE_PACKAGE_NAME="devworkspace-operator"
  DEV_WORKSPACE_SUBSCRIPTION_RECORD=$(oc get subscription -A -o json | jq -r '.items | .[] | select(.spec.name == "'${DEV_WORKSPACE_PACKAGE_NAME}'")')
  DEV_WORKSPACE_SUBSCRIPTION_NAME=$(echo ${DEV_WORKSPACE_SUBSCRIPTION_RECORD} | jq -r '.metadata.name')
  DEV_WORKSPACE_SUBSCRIPTION_NAMESPACE=$(echo ${DEV_WORKSPACE_SUBSCRIPTION_RECORD} | jq -r '.metadata.namespace')
  DEV_WORKSPACE_INSTALLED_CSV=$(echo ${DEV_WORKSPACE_SUBSCRIPTION_RECORD} | jq -r '.status.installedCSV')

  oc delete csv ${DEV_WORKSPACE_INSTALLED_CSV} -n ${DEV_WORKSPACE_SUBSCRIPTION_NAMESPACE}
  oc delete subscription ${DEV_WORKSPACE_SUBSCRIPTION_NAME} -n ${DEV_WORKSPACE_SUBSCRIPTION_NAMESPACE}

  waitForRemovedSubscription ${DEV_WORKSPACE_PACKAGE_NAME}
}

runTests() {
  . ${OPERATOR_REPO}/build/scripts/olm/test-catalog.sh \
      --che-namespace eclipse-che \
      --operator-namespace eclipse-che \
      --catalog-image quay.io/eclipse/eclipse-che-olm-catalog:stable \
      --channel stable \
      --verbose

  deleteEclipseCheStableVersionOperator
  deleteDevWorkspaceStableVersionOperator

  . ${OPERATOR_REPO}/build/scripts/olm/test-catalog-from-sources.sh --verbose
}

runTests
