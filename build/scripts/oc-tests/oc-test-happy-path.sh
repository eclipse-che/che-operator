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

export DEVWORKSPACE_HAPPY_PATH="https://raw.githubusercontent.com/eclipse/che/main/tests/devworkspace-happy-path"
export OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
source "${OPERATOR_REPO}/build/scripts/oc-tests/oc-common.sh"

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
}

usage() {
  echo "Test Eclipse Che happy path (DevWorkspace)"
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
  . "${OPERATOR_REPO}/build/scripts/oc-tests/oc-test-operator.sh" --channel "${CHANNEL}"

  export HAPPY_PATH_USERSTORY=SmokeTest
  export HAPPY_PATH_SUITE=test
  export MOCHA_DIRECTORY='.'
  bash <(curl -s ${DEVWORKSPACE_HAPPY_PATH}/remote-launch.sh)
}

init "$@"
runTests
