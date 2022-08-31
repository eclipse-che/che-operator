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

waitForInstalledEclipseCheCSV() {
  unset ECLIPSE_CHE_INSTALLED_CSV
  while [[ -z ${ECLIPSE_CHE_INSTALLED_CSV} ]] || [[ ${ECLIPSE_CHE_INSTALLED_CSV} == "null" ]]; do
      sleep 5s
      discoverEclipseCheSubscription
  done
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

buildBundleFromSources() {
  local TMP_BUNDLE_DIR=/tmp/bundle
  rm -rf ${TMP_BUNDLE_DIR}
  mkdir ${TMP_BUNDLE_DIR}

  cp -r $(make bundle-path CHANNEL=next)/* ${TMP_BUNDLE_DIR}
  mv ${TMP_BUNDLE_DIR}/bundle.Dockerfile ${TMP_BUNDLE_DIR}/Dockerfile

  # Use custom image in a bundle
  # CI_CHE_OPERATOR_IMAGE it is che operator image built in openshift CI job workflow.
  yq -rYi '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = "'${CI_CHE_OPERATOR_IMAGE}'"' ${TMP_BUNDLE_DIR}/manifests/che-operator.clusterserviceversion.yaml

  oc new-build --binary --strategy docker --name ${BUNDLE_NAME} -n ${NAMESPACE}
  oc start-build ${BUNDLE_NAME} --from-dir ${TMP_BUNDLE_DIR} -n ${NAMESPACE} --wait
}

createRpcCatalogSource() {
  IMAGE_REGISTRY_USER_NAME=kubeadmin
  IMAGE_REGISTRY_USER_TOKEN=$(oc whoami -t)

  oc apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: che-registry
  namespace: ${NAMESPACE}
spec:
  selector:
    matchLabels:
      app: che-registry
  template:
    metadata:
      labels:
        app: che-registry
    spec:
      containers:
      - name: registry
        image: quay.io/openshift-knative/index
        ports:
        - containerPort: 50051
          name: grpc
          protocol: TCP
        livenessProbe:
          exec:
            command:
            - grpc_health_probe
            - -addr=localhost:50051
        readinessProbe:
          exec:
            command:
            - grpc_health_probe
            - -addr=localhost:50051
        command:
        - /bin/sh
        - -c
        - |-
          podman login -u ${IMAGE_REGISTRY_USER_NAME} -p ${IMAGE_REGISTRY_USER_TOKEN} image-registry.openshift-image-registry.svc:5000
          /bin/opm registry add --container-tool=podman -d index.db --mode=semver -b image-registry.openshift-image-registry.svc:5000/${NAMESPACE}/${BUNDLE_NAME}
          /bin/opm registry serve -d index.db -p 50051
EOF

  oc wait --for=condition=ready "pods" -l app=che-registry --timeout=60s -n "${NAMESPACE}"
  local REGISTRY_POD_IP="$(oc get pods -l app=che-registry -n ${NAMESPACE} -o jsonpath='{.items[0].status.podIP}')"

  oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: eclipse-che
  namespace: ${NAMESPACE}
spec:
  address: "${REGISTRY_POD_IP}:50051"
  displayName: "Eclipse Che"
  publisher: "Eclipse Che"
  sourceType: grpc
EOF
}

deployEclipseCheFromSources() {
  make create-namespace NAMESPACE=${NAMESPACE}
  buildBundleFromSources
  createRpcCatalogSource
  make create-operatorgroup NAME=eclipse-che NAMESPACE=${NAMESPACE}
  make create-subscription \
    NAME=eclipse-che-subscription \
    NAMESPACE=eclipse-che \
    PACKAGE_NAME=${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME} \
    SOURCE=eclipse-che \
    SOURCE_NAMESPACE=eclipse-che \
    INSTALL_PLAN_APPROVAL=Auto \
    CHANNEL=next
  waitForInstalledEclipseCheCSV
  getCheClusterCRFromInstalledCSV | oc apply -n "${NAMESPACE}" -f -
  make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE=${NAMESPACE}
}

listCatalogSourceBundles() {
  local name=${1}
  local CATALOG_SERVICE=$(oc get service "${name}" -n openshift-marketplace -o yaml)
  local REGISTRY_IP=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.clusterIP")
  local CATALOG_PORT=$(echo "${CATALOG_SERVICE}" | yq -r ".spec.ports[0].targetPort")

  LIST_BUNDLES=$(oc run grpcurl-query -n openshift-marketplace \
  --rm=true \
  --restart=Never \
  --attach=true \
  --image=docker.io/fullstorydev/grpcurl:v1.7.0 \
  --  -plaintext "${REGISTRY_IP}:${CATALOG_PORT}" api.Registry.ListBundles

}  )

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
# such pull policy. That's why we launch job to force image bundle pulling before Che installation.
forcePullingOlmImages() {
  image="${1}"

  echo "[INFO] Pulling image '${image}'"

  yq -r "(.spec.template.spec.containers[0].image) = \"${image}\"" "${OPERATOR_REPO}/build/scripts/olm/force-pulling-images-job.yaml" | oc apply -f - -n ${NAMESPACE}
  oc wait --for=condition=complete --timeout=30s job/force-pulling-images-job -n ${NAMESPACE}
  oc delete job/force-pulling-images-job -n ${NAMESPACE}
}
