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

set -e

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

init() {
  unset OPERATOR_IMAGE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--help'|'-h') usage; exit;;
      '--operator-image'|'-o') OPERATOR_IMAGE="$2"; shift 1;;
    esac
    shift 1
  done

  if [[ ! ${OPERATOR_IMAGE} ]]; then usage; exit 1; fi
}

usage () {
  echo "Deploy Eclipse Che from sources"
  echo
	echo "Usage:"
	echo -e "\t$0 -o OPERATOR_IMAGE"
  echo
  echo "OPTIONS:"
  echo -e "\t-o,--operator-image      Operator image to include into bundle"
  echo
	echo "Example:"
	echo -e "\t$0 -o quay.io/eclipse/che-operator:next"
}

buildNextBundleFromSources() {
  local TMP_BUNDLE_DIR=/tmp/bundle

  rm -rf ${TMP_BUNDLE_DIR}
  mkdir ${TMP_BUNDLE_DIR}

  cp -r $(make bundle-path CHANNEL=next)/* ${TMP_BUNDLE_DIR}
  mv ${TMP_BUNDLE_DIR}/bundle.Dockerfile ${TMP_BUNDLE_DIR}/Dockerfile

  yq -rYi '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${TMP_BUNDLE_DIR}/manifests/che-operator.clusterserviceversion.yaml

  oc new-build --binary --strategy docker --name ${BUNDLE_NAME} -n ${NAMESPACE}
  oc start-build ${BUNDLE_NAME} --from-dir ${TMP_BUNDLE_DIR} -n ${NAMESPACE} --wait
}

createImageRegistryViewerUser() {
  IMAGE_REGISTRY_VIEWER_USER_NAME=registry-viewer
  IMAGE_REGISTRY_VIEWER_USER_PASSWORD=registry-viewer

  if ! oc get secret registry-viewer-htpasswd -n openshift-config >/dev/null 2>&1; then
    cat > /tmp/htpasswd.conf <<EOF
registry-viewer:{SHA}4xV+Nga1JF5YDx0fB1LdYbyaVvQ=
EOF
    oc create secret generic registry-viewer-htpasswd --from-file=htpasswd=/tmp/htpasswd.conf -n openshift-config
    oc apply -f - <<EOF
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: registry-viewer
    mappingMethod: claim
    type: HTPasswd
    htpasswd:
      fileData:
        name: registry-viewer-htpasswd
EOF
  fi

  IMAGE_REGISTRY_VIEWER_USER_KUBECONFIG=/tmp/${IMAGE_REGISTRY_VIEWER_USER_NAME}.kubeconfig
  if [[ -f "${HOME}/.kube/config" ]]; then
    cp "${HOME}/.kube/config" ${IMAGE_REGISTRY_VIEWER_USER_KUBECONFIG}
  else
    cp "${KUBECONFIG}" ${IMAGE_REGISTRY_VIEWER_USER_KUBECONFIG}
  fi

  timeout 300 bash -c "until oc login --kubeconfig=${IMAGE_REGISTRY_VIEWER_USER_KUBECONFIG}  --username=${IMAGE_REGISTRY_VIEWER_USER_NAME} --password=${IMAGE_REGISTRY_VIEWER_USER_PASSWORD} >/dev/null 2>&1; do printf '.'; sleep 1; done"
  IMAGE_REGISTRY_VIEWER_USER_TOKEN=$(oc --kubeconfig=${IMAGE_REGISTRY_VIEWER_USER_KUBECONFIG} whoami -t)

  oc policy add-role-to-user registry-viewer ${IMAGE_REGISTRY_VIEWER_USER_NAME} -n ${NAMESPACE}
}

createOLMRegistry() {
  oc apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: che-registry
  namespace: ${NAMESPACE}
  labels:
    app: che-registry
  annotations:
    openshift.io/scc: anyuid
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
      podman login -u ${IMAGE_REGISTRY_VIEWER_USER_NAME} -p ${IMAGE_REGISTRY_VIEWER_USER_TOKEN} image-registry.openshift-image-registry.svc:5000
      /bin/opm registry add --container-tool=podman -d index.db --mode=semver -b image-registry.openshift-image-registry.svc:5000/${NAMESPACE}/${BUNDLE_NAME}
      /bin/opm registry serve -d index.db -p 50051
EOF

  oc wait --for=condition=ready "pods" -l app=che-registry --timeout=60s -n "${NAMESPACE}"
}

createEclipseCheCatalogSource() {
  buildNextBundleFromSources
  createImageRegistryViewerUser
  createOLMRegistry

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

run() {
  pushd ${OPERATOR_REPO}
    make create-namespace NAMESPACE=${NAMESPACE}
    if [[ $(oc get operatorgroup -n eclipse-che --no-headers | wc -l) == 0 ]]; then
      make create-operatorgroup NAME=eclipse-che NAMESPACE=${NAMESPACE}
    fi
    createEclipseCheCatalogSource
    make create-subscription \
      NAME=eclipse-che-subscription \
      NAMESPACE=eclipse-che \
      PACKAGE_NAME=${ECLIPSE_CHE_PREVIEW_PACKAGE_NAME} \
      SOURCE=eclipse-che \
      SOURCE_NAMESPACE=eclipse-che \
      INSTALL_PLAN_APPROVAL=Auto \
      CHANNEL=next
    waitForInstalledEclipseCheCSV
    if [[ $(oc get checluster -n eclipse-che --no-headers | wc -l) == 0 ]]; then
      getCheClusterCRFromInstalledCSV | oc apply -n "${NAMESPACE}" -f -
    fi
    make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE=${NAMESPACE}
  popd
}

init "$@"
run

