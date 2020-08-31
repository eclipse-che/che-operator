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

if [ -z "${BASE_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")" && pwd)
fi
ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")

source ${BASE_DIR}/check-yq.sh

incrementNightlyVersion() {
  for platform in 'kubernetes' 'openshift'
  do
    OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
    OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
    CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

    currentNightlyVersion=$(yq -r ".spec.version" "${CSV}")
    echo  "[INFO] current nightly ${platform} version: ${currentNightlyVersion}"

    getNightlyVersionIncrementPart "${currentNightlyVersion}"

    PACKAGE_NAME="eclipse-che-preview-${platform}"
    PACKAGE_FOLDER_PATH="${ROOT_PROJECT_DIR}/olm/eclipse-che-preview-${platform}/deploy/olm-catalog/${PACKAGE_NAME}"
    PACKAGE_FILE_PATH="${PACKAGE_FOLDER_PATH}/${PACKAGE_NAME}.package.yaml"
    CLUSTER_SERVICE_VERSION=$(yq -r ".channels[] | select(.name == \"stable\") | .currentCSV" "${PACKAGE_FILE_PATH}")
    STABLE_PACKAGE_VERSION=$(echo "${CLUSTER_SERVICE_VERSION}" | sed -e "s/${PACKAGE_NAME}.v//")

    newVersion="${STABLE_PACKAGE_VERSION}-$((incrementPart+1)).nightly"

    echo "[INFO] Set up nightly ${platform} version: ${newVersion}"
    yq -rY "(.spec.version) = \"${newVersion}\" | (.metadata.name) = \"eclipse-che-preview-${platform}.v${newVersion}\"" "${CSV}" > "${CSV}.old"
    mv "${CSV}.old" "${CSV}"
  done
}

getNightlyVersionIncrementPart() {
  nightlyVersion="${1}"

  versionWithoutNightly="${nightlyVersion%.nightly}"

  version="${versionWithoutNightly%-*}"

  incrementPart="${versionWithoutNightly#*-}"

  echo "${incrementPart}"
}
