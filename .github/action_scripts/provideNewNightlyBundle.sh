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

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  BASE_DIR=$(cd $(dirname "$0"); pwd)
  ROOT_PROJECT_DIR=$(dirname "$(dirname "${BASE_DIR}")")
fi

cd "${ROOT_PROJECT_DIR}" || exit 1
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
    echo "Should be updated"
fi
done
echo "[INFO] Prepare new bundle."
