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

runTests() {
  # CI_CHE_OPERATOR_IMAGE it is che operator image built in openshift CI job workflow.
  # More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  useCustomOperatorImageInCSV "${CI_CHE_OPERATOR_IMAGE}"

  createNamespace "${NAMESPACE}"
  getCheClusterCRFromCSV | oc apply -n "${NAMESPACE}" -f -
  waitEclipseCheDeployed "$(getCheVersionFromCSV)"
}

initDefaults
runTests
