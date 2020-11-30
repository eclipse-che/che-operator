#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
  ROOT_PROJECT_DIR=$(dirname "$(dirname "${BASE_DIR}")")
fi
export BASE_DIR="${ROOT_PROJECT_DIR}/olm"

# check_che_types function check first if pkg/apis/org/v1/che_types.go file suffer modifications and
# in case of modification should exist also modifications in deploy/crds/* folder.
function check_che_crds() {
    cd "${ROOT_PROJECT_DIR}"
    # CHE_TYPES_FILE make reference to generated code by operator-sdk.
    # Export variables for cr/crds files.
    local CR_CRD_FOLDER="deploy/crds"
    local CR_CRD_REGEX="${CR_CRD_FOLDER}/org_v1_che_crd.yaml"

    # Update crd
    source "${ROOT_PROJECT_DIR}/olm/update-crd-files.sh"

    IFS=$'\n' read -d '' -r -a changedFiles < <( git ls-files -m ) || true
    # Check if there is any difference in the crds. If yes, then fail check.
    if [[ " ${changedFiles[*]} " =~ $CR_CRD_REGEX ]]; then
        echo "[ERROR] CR/CRD file is up to date: ${BASH_REMATCH}. Use 'che-operator/olm/update-crd-files.sh' script to update it."
        exit 1
    else
        echo "[INFO] cr/crd files are in actual state."
    fi
}

installYq() {
  YQ=$(command -v yq) || true
  if [[ ! -x "${YQ}" ]]; then
    pip3 install wheel
    pip3 install yq
    # Make python3 installed modules "visible"
    export PATH=$HOME/.local/bin:$PATH
    ls "${HOME}/.local/bin"
  fi
  echo "[INFO] $(yq --version)"
  echo "[INFO] $(jq --version)"
}

installOperatorSDK() {
  YQ=$(command -v operator-sdk) || true
  if [[ ! -x "${YQ}" ]]; then
    OPERATOR_SDK_TEMP_DIR="$(mktemp -q -d -t "OPERATOR_SDK_XXXXXX" 2>/dev/null || mktemp -q -d)"
    pushd "${OPERATOR_SDK_TEMP_DIR}" || exit
    echo "[INFO] Downloading 'operator-sdk' cli tool..."
    OPERATOR_SDK=$(yq -r ".\"operator-sdk\"" "${ROOT_PROJECT_DIR}/REQUIREMENTS")
    curl -sLo operator-sdk $(curl -sL https://api.github.com/repos/operator-framework/operator-sdk/releases/tags/${OPERATOR_SDK} | jq -r "[.assets[] | select(.name == \"operator-sdk-${OPERATOR_SDK}-x86_64-linux-gnu\")] | first | .browser_download_url")
    export OPERATOR_SDK_BINARY="${OPERATOR_SDK_TEMP_DIR}/operator-sdk"
    chmod +x "${OPERATOR_SDK_BINARY}"
    echo "[INFO] Downloading completed!"
    echo "[INFO] $(${OPERATOR_SDK_BINARY} version)"
    popd || exit
  fi
}

isActualNightlyOlmBundleCSVFiles() {
  cd "${ROOT_PROJECT_DIR}"
  export NO_DATE_UPDATE="true"
  export NO_INCREMENT="true"
  source "${ROOT_PROJECT_DIR}/olm/update-nightly-bundle.sh"

  CSV_FILE_KUBERNETES="deploy/olm-catalog/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml"
  CSV_FILE_OPENSHIFT="deploy/olm-catalog/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"

  IFS=$'\n' read -d '' -r -a changedFiles < <( git ls-files -m ) || true
  for file in "${changedFiles[@]}"
  do
    if [ "${CSV_FILE_KUBERNETES}" == "${file}" ] || [ "${CSV_FILE_OPENSHIFT}" == "${file}" ]; then
      echo "[ERROR] Nightly bundle file ${file} should be updated in your pr, please. Use script 'che-operator/olm/update-nightly-bundle.sh' for this purpose."
      exit 1
    fi
  done
  echo "[INFO] Nightly Olm bundle is in actual state."
}

installYq
installOperatorSDK
check_che_crds
isActualNightlyOlmBundleCSVFiles

echo "[INFO] Done."
