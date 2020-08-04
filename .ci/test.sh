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
go version
ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
# ROOT_PROJECT_DIR=/home/user/GoWorkSpace/src/github.com/eclipse/che-operator
BASE_DIR="${ROOT_PROJECT_DIR}/olm"
echo "${GOPATH}"
# Prevent failing old operator-sdk...
export GOPATH="/home/runner/work/che-operator/go"
echo "${GOPATH}"

# install yq
pip3 install wheel
pip3 install --upgrade setuptools
pip3 install yq
# Make python3 installed modules "visible"
export PATH=$HOME/.local/bin:$PATH
ls "${HOME}/.local/bin"

# install operator SDK.
OPERATOR_SDK_TEMP_DIR="$(mktemp -q -d -t "OPERATOR_SDK_XXXXXX" 2>/dev/null || mktemp -q -d)"
pushd "${OPERATOR_SDK_TEMP_DIR}" || exit
echo "[INFO] Downloading 'operator-sdk' cli tool..."
curl -sLo operator-sdk "$(curl -sL https://api.github.com/repos/operator-framework/operator-sdk/releases/20623716 | jq -r '[.assets[] | select(.name == "operator-sdk-v0.10.1-x86_64-linux-gnu")] | first | .browser_download_url')"
export OPERATOR_SDK_BINARY="${OPERATOR_SDK_TEMP_DIR}/operator-sdk"
chmod +x "${OPERATOR_SDK_BINARY}"
echo "[INFO] Downloading completed!"
"${OPERATOR_SDK_BINARY}" version
popd || exit

CSV_FILE_KUBERNETES="deploy/olm-catalog/che-operator/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml"
CRD_FILE_KUBERNETES="deploy/olm-catalog/che-operator/eclipse-che-preview-kubernetes/manifests/org_v1_che_crd.yaml"
CSV_FILE_OPENSHIFT="deploy/olm-catalog/che-operator/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"
CRD_FILE_OPENSHIFT="deploy/olm-catalog/che-operator/eclipse-che-preview-openshift/manifests/org_v1_che_crd.yaml"
bundle_files=( "${CSV_FILE_KUBERNETES}" "${CRD_FILE_KUBERNETES}" "${CSV_FILE_OPENSHIFT}" "${CRD_FILE_OPENSHIFT}" )

cd ${ROOT_PROJECT_DIR}
source ${ROOT_PROJECT_DIR}/olm/update-nightly-olm-files.sh

echo "======================================"
git ls-files -m
echo "======================================"

# IFS=$'\n' read -d '' -r -a changedFiles < <(git --no-pager diff --name-only HEAD "$(git merge-base HEAD origin/master)") || true
IFS=$'\n' read -d '' -r -a changedFiles < <( git ls-files -m ) || true
echo "Changed files ${changedFiles[*]}"

for file in "${changedFiles[@]}"
do
    echo "======================================"
    echo "${file}"
    echo "======================================"
    # if [ "${CSV_FILE_KUBERNETES}" == "${file}" ] || [ "${CSV_FILE_OPENSHIFT}" == "${file}" ]; then
    #     echo "[ERROR] Nightly bundle file ${file} should be updated in your pr, please. Use script 'che-operator/olm/update-nightly-olm-files.sh' for this purpose."
    #     exit 1
    # fi
    # for bundle_file in "${bundle_files[@]}"
    # do
    #   echo "${bundle_file} vs ${file}"
    #   if [[ "${bundle_file}" == "${file}" ]]; then
    #     echo "[ERROR] Nightly bundle file ${file} should be updated in your pr, please. Use script 'che-operator/olm/update-nightly-olm-files.sh' for this purpose."
    #     exit 1
    #   fi
    # done

    if [[ " ${bundle_files[*]} " =~ ${file} ]]; then
      echo "[ERROR] Nightly bundle file ${file} should be updated in your pr, please. Use script 'che-operator/olm/update-nightly-olm-files.sh' for this purpose."
      exit 1
    fi

done
echo "[INFO] Done."
