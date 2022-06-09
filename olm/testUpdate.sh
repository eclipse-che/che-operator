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

export OPERATOR_REPO="${GITHUB_WORKSPACE}"

if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")")
fi
source "${OPERATOR_REPO}/.github/bin/common.sh"

init() {
  NAMESPACE="eclipse-che"
  CHANNEL="next"
  unset CATALOG_IMAGE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--namespace'|'-n') NAMESPACE="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
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
  echo
	echo "Example:"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -c next"
	echo -e "\t$0 -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -c stable"
}

run() {
  deployDevWorkspaceOperator "${CHANNEL}"

  createNamespace "${NAMESPACE}"
  createCatalogSource "${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" "${CATALOG_IMAGE}"

  local bundles=$(getCatalogSourceBundles "${ECLIPSE_CHE_CATALOG_SOURCE_NAME}")
  fetchPreviousCSVInfo "${CHANNEL}" "${bundles}"
  fetchLatestCSVInfo "${CHANNEL}" "${bundles}"

  if [ "${PREVIOUS_CSV_BUNDLE_IMAGE}" == "${LATEST_CSV_BUNDLE_IMAGE}" ]; then
    echo "[ERROR] Nothing to update. OLM channel '${CHANNEL}' contains only one bundle '${LATEST_CSV_BUNDLE_IMAGE}'"
    exit 1
  fi

  echo "[INFO] Test update from version: ${PREVIOUS_CSV_BUNDLE_IMAGE} to: ${LATEST_CSV_BUNDLE_IMAGE}"
  forcePullingOlmImages "${PREVIOUS_CSV_BUNDLE_IMAGE}"
  forcePullingOlmImages "${LATEST_CSV_BUNDLE_IMAGE}"

  createSubscription "${ECLIPSE_CHE_SUBSCRIPTION_NAME}" "${ECLIPSE_CHE_PACKAGE_NAME}" "${CHANNEL}" "${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" "Manual" "${PREVIOUS_CSV_NAME}"
  approveInstallPlan "${ECLIPSE_CHE_SUBSCRIPTION_NAME}"

  sleep 10s

  getCheClusterCRFromExistedCSV | oc apply -n "${NAMESPACE}" -f -
  waitEclipseCheDeployed "$(getCheVersionFromExistedCSV)"

  approveInstallPlan "${ECLIPSE_CHE_SUBSCRIPTION_NAME}"
  waitEclipseCheDeployed "$(getCheVersionFromExistedCSV)"
}

init "$@"
run

echo "[INFO] Done"
