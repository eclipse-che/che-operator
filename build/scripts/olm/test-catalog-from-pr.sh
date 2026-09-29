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

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

init() {
  unset VERBOSE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--help'|'-h') usage; exit;;
      '--verbose'|'-v') VERBOSE=1;;
    esac
    shift 1
  done
}

usage () {
  echo "Deploy Eclipse Che from the current pull request"
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

run() {
  make create-namespace NAMESPACE="${NAMESPACE}" VERBOSE=${VERBOSE}
  make create-operatorgroup NAME="eclipse-che" NAMESPACE="${NAMESPACE}" VERBOSE=${VERBOSE}

  # Install Dev Workspace operator next version
  make install-devworkspace CHANNEL="next" VERBOSE=${VERBOSE} OPERATOR_NAMESPACE="${NAMESPACE}"

  case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac

  PR_NUMBER=$(gh pr view --json number --jq '.number')
  CATALOG_IMAGE="quay.io/eclipse/eclipse-che-olm-catalog:pr-${PR_NUMBER}-${ARCH}"
  make create-catalogsource NAME="${ECLIPSE_CHE_CATALOG_SOURCE_NAME}" NAMESPACE="${NAMESPACE}" IMAGE="${CATALOG_IMAGE}" VERBOSE=${VERBOSE}

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
  make wait-eclipseche-version VERSION="$(getCheVersionFromInstalledCSV)" NAMESPACE="${NAMESPACE}" VERBOSE=${VERBOSE}
}

init "$@"
[[ ${VERBOSE} == 1 ]] && set -x

pushd "${OPERATOR_REPO}" >/dev/null
run
popd >/dev/null

