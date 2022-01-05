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

# Scripts to prepare OLM(operator lifecycle manager) and install che-operator package
# with specific version using OLM.

BASE_DIR=$(dirname $(readlink -f "${BASH_SOURCE[0]}"))
ROOT_DIR=$(dirname "${BASE_DIR}")

source ${ROOT_DIR}/olm/check-yq.sh

getPackageName() {
  echo "eclipse-che-preview-openshift"
}

getCustomCatalogSourceName() {
  echo "eclipse-che-custom-catalog-source"
}

getSubscriptionName() {
  echo "eclipse-che-subscription"
}

getDevWorkspaceCustomCatalogSourceName() {
  echo "custom-devworkspace-operator-catalog"
}

getBundlePath() {
  channel="${1}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] 'channel' is not specified"
    exit 1
  fi

  echo "${ROOT_DIR}/bundle/${channel}/$(getPackageName)"
}

installOPM() {
  OPM_BINARY=$(command -v opm) || true
  if [[ ! -x $OPM_BINARY ]]; then
    OPM_TEMP_DIR="$(mktemp -q -d -t "OPM_XXXXXX" 2>/dev/null || mktemp -q -d)"
    pushd "${OPM_TEMP_DIR}" || exit

    echo "[INFO] Downloading 'opm' cli tool..."
    curl -sLo opm "$(curl -sL https://api.github.com/repos/operator-framework/operator-registry/releases/33432389 | jq -r '[.assets[] | select(.name == "linux-amd64-opm")] | first | .browser_download_url')"
    export OPM_BINARY="${OPM_TEMP_DIR}/opm"
    chmod +x "${OPM_BINARY}"
    echo "[INFO] Downloading completed!"
    echo "[INFO] 'opm' binary path: ${OPM_BINARY}"
    ${OPM_BINARY} version
    popd || exit
  fi
}

