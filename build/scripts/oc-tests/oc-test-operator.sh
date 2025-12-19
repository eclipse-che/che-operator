#!/bin/bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
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

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

init() {
  unset CHANNEL

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  # Default to 'sources' if not specified,
  [[ ! ${CHANNEL} ]] && CHANNEL="sources"

  # Validate channel parameter
  if [[ "${CHANNEL}" != "stable" ]] && [[ "${CHANNEL}" != "next" ]] && [[ "${CHANNEL}" != "sources" ]]; then
    echo "[ERROR] Invalid channel: ${CHANNEL}. Must be one of: stable, next, sources"
    usage
    exit 1
  fi
}

usage() {
  echo "Test Eclipse Che operator deployment"
  echo
  echo "Usage:"
  echo -e "\t$0 [-c CHANNEL]"
  echo
  echo "OPTIONS:"
  echo -e "\t-c,--channel  [default: sources] Channel to test operator from: stable|next|sources"
  echo -e "\t              - stable: Test from stable catalog (quay.io/eclipse/eclipse-che-olm-catalog:stable)"
  echo -e "\t              - next: Test from next catalog (quay.io/eclipse/eclipse-che-olm-catalog:next)"
  echo -e "\t              - sources: Test from locally built sources"
  echo -e "\t-h,--help     Show this help message"
  echo
  echo "Examples:"
  echo -e "\t$0"
  echo -e "\t$0 -c stable"
  echo -e "\t$0 --channel next"
}

runTests() {
  case "${CHANNEL}" in
    "sources")
      . "${OPERATOR_REPO}/build/scripts/olm/test-catalog-from-sources.sh" --verbose
      ;;
    "stable")
      . ${OPERATOR_REPO}/build/scripts/olm/test-catalog.sh \
        --catalog-image quay.io/eclipse/eclipse-che-olm-catalog:stable \
        --che-namespace eclipse-che \
        --operator-namespace eclipse-che \
        --channel stable \
        --verbose
      ;;
    "next")
      . "${OPERATOR_REPO}/build/scripts/olm/test-catalog.sh" \
        --catalog-image quay.io/eclipse/eclipse-che-olm-catalog:next \
        --che-namespace eclipse-che \
        --operator-namespace eclipse-che \
        --channel next \
        --verbose
      ;;
  esac
}

init "$@"
runTests
