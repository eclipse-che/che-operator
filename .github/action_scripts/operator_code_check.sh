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

# PR_FILES_CHANGED store all Modified/Created files in Pull Request.
export PR_FILES_CHANGED=$(git --no-pager diff --name-only HEAD "$(git merge-base HEAD origin/master)")
echo "========================="
echo "${PR_FILES_CHANGED}"
echo "========================="

# transform_files function transform PR_FILES_CHANGED into a new array => FILES_CHANGED_ARRAY.
function transform_files() {
    for files in ${PR_FILES_CHANGED} 
    do
        FILES_CHANGED_ARRAY+=("${files}")
    done
}

# check_che_types function check first if pkg/apis/org/v1/che_types.go file suffer modifications and
# in case of modification should exist also modifications in deploy/crds/* folder.
function check_che_types() {
    # CHE_TYPES_FILE make reference to generated code by operator-sdk.
    local CHE_TYPES_FILE='pkg/apis/org/v1/che_types.go'
    # Export variables for cr/crds files.
    local CR_CRD_FOLDER="deploy/crds/"
    local CR_CRD_REGEX="\S*org_v1_che_crd.yaml"

    if [[ " ${FILES_CHANGED_ARRAY[*]} " =~ ${CHE_TYPES_FILE} ]]; then
        echo "[INFO] File ${CHE_TYPES_FILE} suffer modifications in PR. Checking if exist modifications for cr/crd files."
        # The script should fail if deploy/crds folder didn't suffer any modification.
        if [[ " ${FILES_CHANGED_ARRAY[*]} " =~ $CR_CRD_REGEX ]]; then
            echo "[INFO] CR/CRD file modified: ${BASH_REMATCH}"
        else
            echo "[ERROR] Detected modification in ${CHE_TYPES_FILE} file, but cr/crd files didn't suffer any modification."
            exit 1
        fi
    else
        echo "[INFO] ${CHE_TYPES_FILE} don't have any modification."
    fi
}

set -e

go version

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
  ROOT_PROJECT_DIR=$(dirname "$(dirname "${BASE_DIR}")")
fi

# Unfortunately ${GOPATH} is required for an old operator-sdk
if [ -z "${GOPATH}" ]; then
    export GOPATH="/home/runner/work/che-operator/go"
    echo "[INFO] GOPATH: ${GOPATH}"
fi

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
    curl -sLo operator-sdk "$(curl -sL https://api.github.com/repos/operator-framework/operator-sdk/releases/19175509 | jq -r '[.assets[] | select(.name == "operator-sdk-v0.10.0-x86_64-linux-gnu")] | first | .browser_download_url')"
    export OPERATOR_SDK_BINARY="${OPERATOR_SDK_TEMP_DIR}/operator-sdk"
    chmod +x "${OPERATOR_SDK_BINARY}"
    echo "[INFO] Downloading completed!"
    echo "[INFO] $(${OPERATOR_SDK_BINARY} version)"
    popd || exit
  fi
}
 
isActualNightlyOlmBundleCSVFiles() {
  cd "${ROOT_PROJECT_DIR}"
  export BASE_DIR="${ROOT_PROJECT_DIR}/olm"
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

transform_files
check_che_types
installYq
installOperatorSDK
isActualNightlyOlmBundleCSVFiles

echo "[INFO] Done."
