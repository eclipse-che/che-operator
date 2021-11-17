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
source "${OPERATOR_REPO}/olm/olm.sh"

init() {
  unset CHANNEL
  unset PLATFORM
  unset CATALOG_IMAGE
  unset NAMESPACE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--platform'|'-p') PLATFORM="$2"; shift 1;;
      '--namespace'|'-n') NAMESPACE="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  if [[ ! ${CHANNEL} ]] || [[ ! ${PLATFORM} ]] || [[ ! ${CATALOG_IMAGE} ]] || [[ ! ${NAMESPACE} ]]; then usage; exit 1; fi
}

usage () {
	echo "Usage:   $0 -p (openshift|kubernetes) -c (next|next-all-namespaces|stable) -i CATALOG_IMAGE -n NAMESPACE"
	echo "Example: $0 -p openshift -c next -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -n eclipse-che"
}

run() {
  createNamespace ${NAMESPACE}
  installOperatorMarketPlace
  installCatalogSource "${PLATFORM}" "${NAMESPACE}" "${CATALOG_IMAGE}"

  getBundleListFromCatalogSource "${PLATFORM}" "${NAMESPACE}"
  getLatestCSVInfo "${CHANNEL}"

  forcePullingOlmImages "${NAMESPACE}" "${LATEST_CSV_BUNDLE_IMAGE}"

  subscribeToInstallation "${PLATFORM}" "${NAMESPACE}" "${CHANNEL}" "${LATEST_CSV_NAME}"
  installPackage "${PLATFORM}" "${NAMESPACE}"

  applyCheClusterCR ${LATEST_CSV_NAME} ${PLATFORM}
  waitCheServerDeploy "${NAMESPACE}"
}

init $@
installOPM
run

echo "[INFO] Done"

