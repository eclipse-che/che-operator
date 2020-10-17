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

set -ex

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  BASE_DIR=$(cd $(dirname "$0"); pwd)
  ROOT_PROJECT_DIR=$(dirname "$(dirname "${BASE_DIR}")")
fi

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

updateDate() {
    NEW_CSV=${1}
    createdAt=$(date -u +%FT%TZ)
    echo "[INFO]        - createdAt => ${createdAt}"
    sed -e "s/createdAt:.*$/createdAt: \"${createdAt}\"/" "${NEW_CSV}" > "${NEW_CSV}.new"
    mv "${NEW_CSV}.new" "${NEW_CSV}"
}

installYq
installOperatorSDK

# Update nightly bundles, but without time and increment to detect if we really have changes for new bundles.
export BASE_DIR="${ROOT_PROJECT_DIR}/olm"
export NO_DATE_UPDATE="true"
export NO_INCREMENT="true"
source "${ROOT_PROJECT_DIR}/olm/update-nightly-bundle.sh"

source "${ROOT_PROJECT_DIR}/olm/incrementNightlyBundles.sh"

CSV_FILE_KUBERNETES="deploy/olm-catalog/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml"
CSV_FILE_OPENSHIFT="deploy/olm-catalog/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"

IFS=$'\n' read -d '' -r -a changedFiles < <( git ls-files -m ) || true
for file in "${changedFiles[@]}"
do
  if [ "${CSV_FILE_KUBERNETES}" == "${file}" ]; then
    incrementNightlyVersion 'kubernetes'
    updateDate "${CSV_FILE_KUBERNETES}"
    echo "[INFO] Prepared new bundle file ${CSV_FILE_KUBERNETES}"
  fi

  if [ "${CSV_FILE_OPENSHIFT}" == "${file}" ]; then
    incrementNightlyVersion 'openshift'
    updateDate "${CSV_FILE_OPENSHIFT}"
    echo "[INFO] Prepared new bundle file ${CSV_FILE_OPENSHIFT}"
  fi
done
echo "[INFO] Done."
