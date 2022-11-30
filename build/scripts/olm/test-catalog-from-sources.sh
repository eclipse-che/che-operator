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

export ECLIPSE_CHE_ROOT_DIR=/tmp/eclipse-che
export CATALOG_DIR=${ECLIPSE_CHE_ROOT_DIR}/olm-catalog/next
export BUNDLE_DIR=${ECLIPSE_CHE_ROOT_DIR}/bundle
export CA_DIR=${ECLIPSE_CHE_ROOT_DIR}/certificates
export BUNDLE_NAME=$(make bundle-name CHANNEL=next)

# Images names in the OpenShift registry
export REGISTRY_BUNDLE_IMAGE_NAME="eclipse-che-bundle"
export REGISTRY_CATALOG_IMAGE_NAME="eclipse-che-catalog"
export REGISTRY_OPERATOR_IMAGE_NAME="eclipse-che-operator"

# Images
unset OPERATOR_IMAGE
unset BUNDLE_IMAGE
unset CATALOG_IMAGE

init() {
  unset VERBOSE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--help'|'-h') usage; exit;;
      '--verbose'|'-v') VERBOSE=1;;
    esac
    shift 1
  done

  rm -rf ${ECLIPSE_CHE_ROOT_DIR}
  mkdir -p ${CATALOG_DIR}
  mkdir -p ${BUNDLE_DIR}
  mkdir -p ${CA_DIR}
}

usage () {
  echo "Deploy Eclipse Che from sources"
  echo
	echo "Usage:"
	echo -e "\t$0 [--verbose]"
  echo
  echo "OPTIONS:"
  echo -e "\t-v,--verbose             Verbose mode"
  echo
	echo "Example:"
	echo -e "\t$0"
}

exposeOpenShiftRegistry() {
  oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
  sleep 5s
  REGISTRY_HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')

  BUNDLE_IMAGE="${REGISTRY_HOST}/${NAMESPACE}/${REGISTRY_BUNDLE_IMAGE_NAME}:latest"
  echo "[INFO] Bundle image: ${BUNDLE_IMAGE}"

  OPERATOR_IMAGE="${REGISTRY_HOST}/${NAMESPACE}/${REGISTRY_OPERATOR_IMAGE_NAME}:latest"
  echo "[INFO] Operator image: ${OPERATOR_IMAGE}"

  CATALOG_IMAGE="${REGISTRY_HOST}/${NAMESPACE}/${REGISTRY_CATALOG_IMAGE_NAME}:latest"
  echo "[INFO] Catalog image: ${CATALOG_IMAGE}"

  oc get secret -n openshift-ingress  router-certs-default -o go-template='{{index .data "tls.crt"}}' | base64 -d > ${CA_DIR}/ca.crt

  oc delete configmap openshift-registry --ignore-not-found=true -n openshift-config
  oc create configmap openshift-registry -n openshift-config --from-file=${REGISTRY_HOST}=${CA_DIR}/ca.crt

  oc patch image.config.openshift.io/cluster --patch '{"spec":{"additionalTrustedCA":{"name":"openshift-registry"}}}' --type=merge

  oc policy add-role-to-user system:image-builder system:anonymous -n "${NAMESPACE}"
  oc policy add-role-to-user system:image-builder system:unauthenticated -n "${NAMESPACE}"
}

buildOperatorFromSources() {
  oc delete buildconfigs ${REGISTRY_OPERATOR_IMAGE_NAME} --ignore-not-found=true -n "${NAMESPACE}"
  oc delete imagestreamtag ${REGISTRY_OPERATOR_IMAGE_NAME}:latest --ignore-not-found=true -n "${NAMESPACE}"

  oc new-build --binary --strategy docker --name "${REGISTRY_OPERATOR_IMAGE_NAME}" -n "${NAMESPACE}"
  oc start-build "${REGISTRY_OPERATOR_IMAGE_NAME}" --from-dir "${OPERATOR_REPO}" -n "${NAMESPACE}" --wait
}

buildBundleFromSources() {
  cp -r $(make bundle-path CHANNEL=next)/* ${BUNDLE_DIR}
  mv ${BUNDLE_DIR}/bundle.Dockerfile ${BUNDLE_DIR}/Dockerfile

  # Set operator image from the registry
  yq -rYi '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${BUNDLE_DIR}/manifests/che-operator.clusterserviceversion.yaml

  oc delete buildconfigs ${REGISTRY_BUNDLE_IMAGE_NAME} --ignore-not-found=true -n "${NAMESPACE}"
  oc delete imagestreamtag ${REGISTRY_BUNDLE_IMAGE_NAME}:latest --ignore-not-found=true -n "${NAMESPACE}"

  oc new-build --binary --strategy docker --name "${REGISTRY_BUNDLE_IMAGE_NAME}" -n "${NAMESPACE}"
  oc start-build "${REGISTRY_BUNDLE_IMAGE_NAME}" --from-dir ${BUNDLE_DIR} -n "${NAMESPACE}" --wait
}

buildCatalogFromSources() {
  cat > ${CATALOG_DIR}/package.yaml <<EOF
schema: olm.package
name: eclipse-che
defaultChannel: next
EOF

  cat > ${CATALOG_DIR}/channel.yaml <<EOF
schema: olm.channel
package: eclipse-che
name: next
entries:
  - name: ${BUNDLE_NAME}
EOF

  make bundle-render CHANNEL=next BUNDLE_NAME="${BUNDLE_NAME}" CATALOG_DIR="${CATALOG_DIR}" BUNDLE_IMG="${BUNDLE_IMAGE}" VERBOSE=${VERBOSE}
  cp "${OPERATOR_REPO}/olm-catalog/index.Dockerfile" $(dirname "${CATALOG_DIR}")/Dockerfile
  sed -i 's|ADD olm-catalog/${CHANNEL}|ADD '$(basename ${CATALOG_DIR})'|g' $(dirname "${CATALOG_DIR}")/Dockerfile

  oc delete buildconfigs ${REGISTRY_CATALOG_IMAGE_NAME} --ignore-not-found=true -n "${NAMESPACE}"
  oc delete imagestreamtag ${REGISTRY_CATALOG_IMAGE_NAME}:latest --ignore-not-found=true -n "${NAMESPACE}"

  oc new-build --binary --strategy docker --name "${REGISTRY_CATALOG_IMAGE_NAME}" -n "${NAMESPACE}"
  oc start-build "${REGISTRY_CATALOG_IMAGE_NAME}" --from-dir $(dirname ${CATALOG_DIR}) -n "${NAMESPACE}" --wait
}

createEclipseCheCatalogFromSources() {
  buildOperatorFromSources
  buildBundleFromSources
  buildCatalogFromSources
  make create-catalogsource NAME="${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" NAMESPACE="${NAMESPACE}" IMAGE="${CATALOG_IMAGE}" VERBOSE=${VERBOSE}
}

run() {
  make create-namespace NAMESPACE="${NAMESPACE}" VERBOSE=${VERBOSE}

  # Install Dev Workspace operator (next version as well)
  make install-devworkspace CHANNEL="next"

  exposeOpenShiftRegistry
  createEclipseCheCatalogFromSources

  if [[ $(oc get operatorgroup -n "${NAMESPACE}" --no-headers | wc -l) == 0 ]]; then
    make create-operatorgroup NAME=eclipse-che NAMESPACE="${NAMESPACE}" VERBOSE=${VERBOSE}
  fi
  make create-subscription \
    NAME=eclipse-che \
    NAMESPACE="${NAMESPACE}" \
    PACKAGE_NAME="${ECLIPSE_CHE_PACKAGE_NAME}" \
    SOURCE="${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" \
    SOURCE_NAMESPACE="${NAMESPACE}" \
    INSTALL_PLAN_APPROVAL=Auto \
    CHANNEL=next \
    VERBOSE=${VERBOSE}
  make wait-pod-running NAMESPACE="${NAMESPACE}" SELECTOR="app.kubernetes.io/component=che-operator"

  if [[ $(oc get checluster -n eclipse-che --no-headers | wc -l) == 0 ]]; then
    getCheClusterCRFromInstalledCSV | oc apply -n "${NAMESPACE}" -f -
  fi
  make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE="${NAMESPACE}" VERBOSE=${VERBOSE}
}

init "$@"
[[ ${VERBOSE} == 1 ]] && set -x

pushd "${OPERATOR_REPO}" >/dev/null
run
popd >/dev/null

