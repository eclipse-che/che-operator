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

export ECLIPSE_CHE_PACKAGE_NAME="eclipse-che-preview-openshift"
export ECLIPSE_CHE_CATALOG_SOURCE_NAME="eclipse-che-custom-catalog-source"
export ECLIPSE_CHE_SUBSCRIPTION_NAME="eclipse-che-subscription"

useCustomOperatorImageInCSV() {
  local OPERATOR_IMAGE=$1
  local ECLIPSE_CHE_SUBSCRIPTION_RECORD=($(oc get subscription -A | grep eclipse-che))
  local ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD[0]})
  local ECLIPSE_CHE_SUBSCRIPTION_NAME=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD[1]})
  local ECLIPSE_CHE_INSTALLED_CSV=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o jsonpath="{.status.installedCSV}")

  oc patch ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} --type=json -p '[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "'${OPERATOR_IMAGE}'"}]'
}

getCheClusterCRFromExistedCSV() {
  local ECLIPSE_CHE_SUBSCRIPTION_RECORD=($(oc get subscription -A | grep eclipse-che))
  local ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD[0]})
  local ECLIPSE_CHE_SUBSCRIPTION_NAME=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD[1]})
  local ECLIPSE_CHE_INSTALLED_CSV=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o jsonpath="{.status.installedCSV}")

  oc get csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o yaml | yq -r '.metadata.annotations["alm-examples"] | fromjson | .[] | select(.apiVersion == "org.eclipse.che/v2")'
}

getCheVersionFromExistedCSV() {
  local ECLIPSE_CHE_SUBSCRIPTION_RECORD=($(oc get subscription -A | grep eclipse-che))
  local ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD[0]})
  local ECLIPSE_CHE_SUBSCRIPTION_NAME=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD[1]})
  local ECLIPSE_CHE_INSTALLED_CSV=$(oc get subscription ${ECLIPSE_CHE_SUBSCRIPTION_NAME} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o jsonpath="{.status.installedCSV}")

  oc get csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o yaml | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value'
}