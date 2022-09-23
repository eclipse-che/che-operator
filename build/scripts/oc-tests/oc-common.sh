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

export NAMESPACE="eclipse-che"
export BUNDLE_NAME="che-bundle"
export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/artifacts-che"}
export ECLIPSE_CHE_STABLE_PACKAGE_NAME="eclipse-che"
export ECLIPSE_CHE_PREVIEW_PACKAGE_NAME="eclipse-che-preview-openshift"
export ECLIPSE_CHE_CATALOG_SOURCE_NAME="eclipse-che-custom-catalog-source"
export ECLIPSE_CHE_SUBSCRIPTION_NAME="eclipse-che-subscription"

catchFinish() {
  local RESULT=$?

  # Collect all Eclipse Che logs
  set +e && chectl server:logs -n $NAMESPACE -d $ARTIFACTS_DIR --telemetry off && set -e

  [[ "${RESULT}" != "0" ]] && echo "[ERROR] Job failed." || echo "[INFO] Job completed successfully."
  rm -rf ${OPERATOR_REPO}/tmp

  exit ${RESULT}
}

waitForRemovedEclipseCheSubscription() {
  while [[ $(oc get subscription -A -o json | jq -r '.items | .[] | select(.spec.name == "'${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME}'" or .spec.name == "'${ECLIPSE_CHE_STABLE_PACKAGE_NAME}'")') != "" ]]; do
      sleep 5s
  done
}

useCustomOperatorImageInCSV() {
  local OPERATOR_IMAGE=$1
  discoverEclipseCheSubscription
  oc patch csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} --type=json -p '[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "'${OPERATOR_IMAGE}'"}]'
}

getCheClusterCRFromInstalledCSV() {
  discoverEclipseCheSubscription
  oc get csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o yaml | yq -r '.metadata.annotations["alm-examples"] | fromjson | .[] | select(.apiVersion == "org.eclipse.che/v2")'
}

getCheVersionFromInstalledCSV() {
  discoverEclipseCheSubscription
  oc get csv ${ECLIPSE_CHE_INSTALLED_CSV} -n ${ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE} -o yaml | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value'
}

discoverEclipseCheSubscription() {
  ECLIPSE_CHE_SUBSCRIPTION_RECORD=$(oc get subscription -A -o json | jq -r '.items | .[] | select(.spec.name == "'${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME}'" or .spec.name == "'${ECLIPSE_CHE_STABLE_PACKAGE_NAME}'")')
  ECLIPSE_CHE_SUBSCRIPTION_NAME=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.metadata.name')
  ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.metadata.namespace')
  ECLIPSE_CHE_INSTALLED_CSV=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.status.installedCSV')
}

discoverEclipseCheBundles() {
  local CHANNEL=$1
  local CATALOG_SERVICE=$(oc get service ${ECLIPSE_CHE_CATALOG_SOURCE_NAME} -n openshift-marketplace -o yaml)
  local REGISTRY_IP=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.clusterIP")
  local CATALOG_PORT=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.ports[0].targetPort")

  local xFlag="+x"; [[ $- =~ x ]] && xFlag="-x"
  set +x # suppress output
  local BUNDLES=$(oc run grpcurl-query -n openshift-marketplace \
  --rm=true \
  --restart=Never \
  --attach=true \
  --image=docker.io/fullstorydev/grpcurl:v1.7.0 \
  --  -plaintext "${REGISTRY_IP}:${CATALOG_PORT}" api.Registry.ListBundles | head -n -1)

  local LATEST_BUNDLE=$(echo "${BUNDLES}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${CHANNEL}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 1]')
  local PREVIOUS_BUNDLE=$(echo "${BUNDLES}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${CHANNEL}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 2]')

  export LATEST_CSV_NAME=$(echo "${LATEST_BUNDLE}" | yq -r ".csvName")
  export PREVIOUS_CSV_NAME=$(echo "${PREVIOUS_BUNDLE}" | yq -r ".csvName")
  if [[ ${CHANNEL} == "next" ]]; then
    export LATEST_VERSION="next"
    export PREVIOUS_VERSION="next"
  else
    export LATEST_VERSION=${LATEST_CSV_NAME#${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME}.v}
    export PREVIOUS_VERSION=${PREVIOUS_CSV_NAME#${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME}.v}
  fi

  echo "[INFO] PREVIOUS_CSV_NAME:         ${PREVIOUS_CSV_NAME}"
  echo "[INFO] PREVIOUS_VERSION:          ${PREVIOUS_VERSION}"
  echo "[INFO] LATEST_CSV_NAME:           ${LATEST_CSV_NAME}"
  echo "[INFO] LATEST_VERSION:            ${LATEST_VERSION}"
  set ${xFlag}
}
