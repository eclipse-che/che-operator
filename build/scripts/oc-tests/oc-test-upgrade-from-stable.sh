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

trap "catchFinish" EXIT SIGINT

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
  echo -e "  --to-channel CHANNEL  Channel to test the operator upgrade to."
  echo -e "                        next    - to the next catalog"
  echo -e "                        pr      - to the current pull request"
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
  PR_NUMBER=$(gh pr view --json number --jq '.number')

  case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac

  make create-catalogsource NAME="eclipse-che-update" \
    NAMESPACE="openshift-marketplace" \
    IMAGE="quay.io/eclipse/eclipse-che-olm-catalog:pr-${PR_NUMBER}-${ARCH}"
}

createEclipseCheCatalogSourceFromNext() {
  make create-catalogsource NAME="eclipse-che-update" \
    NAMESPACE="openshift-marketplace" \
    IMAGE="quay.io/eclipse/eclipse-che-olm-catalog:next"
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

  oc patch subscription "eclipse-che" -n "openshift-operators" --type=merge -p '{"spec":{"channel":"next","source":"eclipse-che-update"}}'
  make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE="eclipse-che"
}

init "$@"
runTests
