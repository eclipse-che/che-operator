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
export ECLIPSE_CHE_PREVIEW_PACKAGE_NAME="eclipse-che-preview-openshift"
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
  ECLIPSE_CHE_SUBSCRIPTION_RECORD=$(oc get subscription -A -o json | jq -r '.items | .[] | select(.spec.name == "'${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME}'" or .spec.name == "'${ECLIPSE_CHE_STABLE_PACKAGE_NAME}'")')
  ECLIPSE_CHE_SUBSCRIPTION_NAME=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.metadata.name')
  ECLIPSE_CHE_SUBSCRIPTION_NAMESPACE=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.metadata.namespace')
  ECLIPSE_CHE_INSTALLED_CSV=$(echo ${ECLIPSE_CHE_SUBSCRIPTION_RECORD} | jq -r '.status.installedCSV')
}

getCatalogSourceBundles() {
  local name=${1}
  local catalogService=$(kubectl get service "${name}" -n openshift-marketplace -o yaml)
  local catalogIP=$(echo "${catalogService}" | yq -r ".spec.clusterIP")
  local catalogPort=$(echo "${catalogService}" | yq -r ".spec.ports[0].targetPort")

  LIST_BUNDLES=$(kubectl run grpcurl-query -n openshift-marketplace \
  --rm=true \
  --restart=Never \
  --attach=true \
  --image=docker.io/fullstorydev/grpcurl:v1.7.0 \
  --  -plaintext "${catalogIP}:${catalogPort}" api.Registry.ListBundles
  )

  echo "${LIST_BUNDLES}" | head -n -1
}

fetchPreviousCSVInfo() {
  local channel="${1}"
  local bundles="${2}"

  previousBundle=$(echo "${bundles}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 2]')
  export PREVIOUS_CSV_NAME=$(echo "${previousBundle}" | yq -r ".csvName")
  if [ "${PREVIOUS_CSV_NAME}" == "null" ]; then
    echo "[ERROR] Catalog source image hasn't got previous bundle."
    exit 1
  fi
  export PREVIOUS_CSV_BUNDLE_IMAGE=$(echo "${previousBundle}" | yq -r ".bundlePath")
}

fetchLatestCSVInfo() {
  local channel="${1}"
  local bundles="${2}"

  latestBundle=$(echo "${bundles}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 1]')
  export LATEST_CSV_NAME=$(echo "${latestBundle}" | yq -r ".csvName")
  export LATEST_CSV_BUNDLE_IMAGE=$(echo "${latestBundle}" | yq -r ".bundlePath")
}

# HACK. Unfortunately catalog source image bundle job has image pull policy "IfNotPresent".
# It makes troubles for test scripts, because image bundle could be outdated with
# such pull policy. That's why we launch job to fource image bundle pulling before Che installation.
forcePullingOlmImages() {
  image="${1}"

  echo "[INFO] Pulling image '${image}'"

  yq -r "(.spec.template.spec.containers[0].image) = \"${image}\"" "${OPERATOR_REPO}/olm/force-pulling-olm-images-job.yaml" | kubectl apply -f - -n openshift-marketplace

  kubectl wait --for=condition=complete --timeout=30s job/force-pulling-olm-images-job -n openshift-marketplace
  kubectl delete job/force-pulling-olm-images-job -n openshift-marketplace
}
