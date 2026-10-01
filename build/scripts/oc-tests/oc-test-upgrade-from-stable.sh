#!/bin/bash
#
# Copyright (c) 2019-2026 Red Hat, Inc.
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

export OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

trap "catchFinish" EXIT
trap 'exit 130' SIGINT

init() {
  unset TO_CHANNEL

  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      --to-channel)
        TO_CHANNEL="$2"
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
    esac

    shift
  done

  if [[ "$TO_CHANNEL" != "next" && "$TO_CHANNEL" != "pr" ]]; then
    echo "[ERROR] Invalid --to-channel flag. Channel must be one of: next, pr"
    usage
    exit 1
  fi
}

usage() {
  echo "Test Eclipse Che upgrade path"
  echo
  echo "Usage:"
  echo -e "  $0 [--to-channel CHANNEL]"
  echo
  echo "OPTIONS:"
  echo -e "  --to-channel CHANNEL  Channel to test the operator upgrade path to."
  echo -e "                        next    - to the next version"
  echo -e "                        pr      - to the the pull request version "
  echo -e "  -h, --help            Show this help message"
}

installDevWorkspacesStableVersion() {
  make install-devworkspace \
    CHANNEL="fast" \
    OPERATOR_NAMESPACE="openshift-operators"
}

installEclipseCheStableVersion() {
  make create-catalogsource NAME="eclipse-che" \
    IMAGE="quay.io/eclipse/eclipse-che-olm-catalog:stable" \
    NAMESPACE="openshift-marketplace"

  make create-subscription \
    NAME="eclipse-che" \
    NAMESPACE="openshift-operators" \
    PACKAGE_NAME="${ECLIPSE_CHE_PACKAGE_NAME}" \
    CHANNEL="stable" \
    SOURCE="eclipse-che" \
    SOURCE_NAMESPACE="openshift-marketplace" \
    INSTALL_PLAN_APPROVAL="Auto"

  make wait-pod-running \
    NAMESPACE="openshift-operators" \
    SELECTOR="app.kubernetes.io/component=che-operator"

  make create-namespace NAMESPACE="eclipse-che"
  getCheClusterCRFromInstalledCSV | oc apply --server-side -n "eclipse-che" -f -
  make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE="eclipse-che"
}

createEclipseCheCatalogSourceFromPR() {
  local catalog_image=$(getCatalogImageFromPullRequest)

  make create-catalogsource NAME="eclipse-che-update" \
    NAMESPACE="openshift-marketplace" \
    IMAGE="${catalog_image}"
}

createEclipseCheCatalogSourceFromNext() {
  make create-catalogsource NAME="eclipse-che-update" \
    NAMESPACE="openshift-marketplace" \
    IMAGE="quay.io/eclipse/eclipse-che-olm-catalog:next"
}

updateEclipseChe() {
    INSTALLED_CSV=$(oc get subscription eclipse-che -n openshift-operators -o jsonpath='{.status.installedCSV}')
    oc patch subscription "eclipse-che" -n "openshift-operators" --type=merge -p '{"spec":{"channel":"next","source":"eclipse-che-update"}}'

    # Wait for OLM to pick up the new catalog and advance the CSV
    timeout 120s bash -c '
      until [[ "$(oc get subscription eclipse-che -n openshift-operators -o jsonpath="{.status.installedCSV}")" != "$1" ]]; do
        sleep 5
      done
    ' _ "${INSTALLED_CSV}" || {
      echo "[ERROR] Timed out waiting for installedCSV to change from ${INSTALLED_CSV}"
      exit 1
    }
    make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE="eclipse-che"
}

runTests() {
  installDevWorkspacesStableVersion
  installEclipseCheStableVersion

  case "${TO_CHANNEL}" in
    "pr")
      createEclipseCheCatalogSourceFromPR
      ;;
    "next")
      createEclipseCheCatalogSourceFromNext
      ;;
  esac

  updateEclipseChe
}

init "$@"
runTests
