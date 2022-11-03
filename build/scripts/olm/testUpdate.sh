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
  NAMESPACE="eclipse-che"
  CHANNEL="next"
  unset VERBOSE
  unset CATALOG_IMAGE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--namespace'|'-n') NAMESPACE="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--verbose'|'-v') VERBOSE=1;;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  if [[ ! ${CHANNEL} ]] || [[ ! ${CATALOG_IMAGE} ]]; then usage; exit 1; fi
}

usage () {
  echo "Deploy and update Eclipse Che from a custom catalog."
  echo
	echo "Usage:"
	echo -e "\t$0 -i CATALOG_IMAGE [-c CHANNEL] [-n NAMESPACE]"
  echo
  echo "OPTIONS:"
  echo -e "\t-i,--catalog-image       Catalog image"
  echo -e "\t-c,--channel             [default: next] Olm channel to deploy Eclipse Che from"
  echo -e "\t-n,--namespace           [default: eclipse-che] Kubernetes namepsace to deploy Eclipse Che into"
  echo -e "\t-v,--verbose             Verbose mode"
  echo
	echo "Example:"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -c next"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -c stable"
}

run() {
  make create-namespace NAMESPACE="eclipse-che" VERBOSE=${VERBOSE}
  make create-catalogsource NAME="${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" IMAGE="${CATALOG_IMAGE}" VERBOSE=${VERBOSE}

  discoverEclipseCheBundles ${CHANNEL}

  if [[ "${PREVIOUS_CSV_NAME}" == "${LATEST_CSV_NAME}" ]]; then
    echo "[ERROR] Nothing to update. OLM channel '${CHANNEL}' contains only one bundle '${LATEST_CSV_NAME}'"
    exit 1
  fi

  if [[ "${LATEST_VERSION}" == "null" ]]; then
    echo "[ERROR] CatalogSource does not contain any bundles."
    exit 1
  fi

  if [[ "${PREVIOUS_VERSION}" == "null" ]]; then
    echo "[ERROR] CatalogSource contains only one bundle."
    exit 1
  fi

  if [[ ${CHANNEL} == "next" ]]; then
    make install-devworkspace CHANNEL=next VERBOSE=${VERBOSE}
  else
    make install-devworkspace CHANNEL=fast VERBOSE=${VERBOSE}
  fi

  echo "[INFO] Test update from version: ${PREVIOUS_VERSION} to: ${LASTEST_VERSION}"
  make create-subscription \
    NAME="${ECLIPSE_CHE_SUBSCRIPTION_NAME}" \
    NAMESPACE="openshift-operators" \
    PACKAGE_NAME="${ECLIPSE_CHE_PACKAGE_NAME}" \
    CHANNEL="${CHANNEL}" \
    SOURCE="${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" \
    SOURCE_NAMESPACE="openshift-marketplace" \
    INSTALL_PLAN_APPROVAL="Manual" \
    STARTING_CSV="${PREVIOUS_CSV_NAME}" \
    VERBOSE=${VERBOSE}
  make approve-installplan SUBSCRIPTION_NAME="${ECLIPSE_CHE_SUBSCRIPTION_NAME}" NAMESPACE="openshift-operators"
  make wait-pod-running NAMESPACE="openshift-operators" SELECTOR="app.kubernetes.io/component=che-operator"

  getCheClusterCRFromInstalledCSV | oc apply -n "${NAMESPACE}" -f -
  make wait-eclipseche-version VERSION=${PREVIOUS_VERSION} NAMESPACE=${NAMESPACE} VERBOSE=${VERBOSE}

  make approve-installplan SUBSCRIPTION_NAME="${ECLIPSE_CHE_SUBSCRIPTION_NAME}" NAMESPACE="openshift-operators" VERBOSE=${VERBOSE}
  make wait-eclipseche-version VERSION=${LATEST_VERSION} NAMESPACE=${NAMESPACE} VERBOSE=${VERBOSE}
}

init "$@"
[[ ${VERBOSE} == 1 ]] && set -x

pushd ${OPERATOR_REPO} >/dev/null
run
popd >/dev/null

echo "[INFO] Done"
