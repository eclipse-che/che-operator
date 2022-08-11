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

export ECLIPSE_CHE_STABLE_PACKAGE_NAME="eclipse-che"
export ECLIPSE_CHE_NEXT_PACKAGE_NAME="eclipse-che-preview-openshift"
export ECLIPSE_CHE_CATALOG_SOURCE_NAME="eclipse-che-custom-catalog-source"
export ECLIPSE_CHE_SUBSCRIPTION_NAME="eclipse-che-subscription"

useCustomOperatorImageInCSV() {
  local OPERATOR_IMAGE=$1
  findEclipseCheSubscription
  oc patch csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} --type=json -p '[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "'${OPERATOR_IMAGE}'"}]'
}

getCheClusterCRFromCSV() {
  findEclipseCheSubscription
  oc get csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o yaml | yq -r '.metadata.annotations["alm-examples"] | fromjson | .[] | select(.apiVersion == "org.eclipse.che/v2")'
}

getCheVersionFromCSV() {
  findEclipseCheSubscription
  oc get csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o yaml | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value'
}

findEclipseCheSubscription() {
  ECLIPSE_CHE_SUBSCRIPTION_RECORD=$(oc get subscription -A -o json | jq -r '.items | .[] | select(.spec.name == "'${ECLIPSE_CHE_NEXT_PACKAGE_NAME}'" or .spec.name == "'${ECLIPSE_CHE_STABLE_PACKAGE_NAME}'")')
  ECLIPSE_CHE_SUBSCRIPTION_NAME=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.metadata.name')
  ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.metadata.namespace')
  ECLIPSE_CHE_INSTALLED_CSV=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.status.installedCSV')
}