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

export OPERATOR_REPO="${GITHUB_WORKSPACE}"

if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")")
fi
source "${OPERATOR_REPO}"/olm/olm.sh

init() {
  unset CHANNEL
  unset CATALOG_IMAGE
  unset OPERATOR_IMAGE
  unset NAMESPACE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--namespace'|'-n') NAMESPACE="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  if [[ ! ${CHANNEL} ]] || [[ ! ${CATALOG_IMAGE} ]] || [[ ! ${NAMESPACE} ]]; then usage; exit 1; fi
}

usage () {
	echo "Usage:   $0 -c (next|stable) -i CATALOG_IMAGE -n NAMESPACE"
	echo "Example: $0 -c next -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -n eclipse-che"
}

run() {
  createNamespace "${NAMESPACE}"
  installOperatorMarketPlace
  installCatalogSource "${NAMESPACE}" "${CATALOG_IMAGE}"

  getBundleListFromCatalogSource "${NAMESPACE}"
  getPreviousCSVInfo "${CHANNEL}"
  getLatestCSVInfo "${CHANNEL}"

  echo "[INFO] Test update from version: ${PREVIOUS_CSV_BUNDLE_IMAGE} to: ${LATEST_CSV_BUNDLE_IMAGE}"

  if [ "${PREVIOUS_CSV_BUNDLE_IMAGE}" == "${LATEST_CSV_BUNDLE_IMAGE}" ]; then
    echo "[ERROR] Nothing to update. OLM channel '${channel}' contains only one bundle."
    exit 1
  fi

  forcePullingOlmImages "${NAMESPACE}" "${PREVIOUS_CSV_BUNDLE_IMAGE}"
  forcePullingOlmImages "${NAMESPACE}" "${LATEST_CSV_BUNDLE_IMAGE}"

  subscribeToInstallation "${NAMESPACE}" "${CHANNEL}" "${PREVIOUS_CSV_NAME}"
  installPackage "${NAMESPACE}"
  echo "[INFO] Installation of the previous che-operator version: ${PREVIOUS_CSV_NAME} successfully completed"

  applyCheClusterCR ${PREVIOUS_CSV_NAME}
  waitCheServerDeploy "${NAMESPACE}"

  installPackage "${NAMESPACE}"
  echo "[INFO] Installation of the latest che-operator version: ${LATEST_CSV_NAME} successfully completed"
}

init "$@"
run
