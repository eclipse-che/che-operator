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

source ${BASE_DIR}/check-yq.sh
source ${BASE_DIR}/olm.sh

incrementNightlyVersion() {
  platform="${1}"
  if [ -z "${platform}" ]; then
    echo "[ERROR] please specify first argument 'platform'"
    exit 1
  fi

  NIGHTLY_BUNDLE_PATH=$(getBundlePath "${platform}" "nightly")
  OPM_BUNDLE_MANIFESTS_DIR="${NIGHTLY_BUNDLE_PATH}/manifests"
  CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

  currentNightlyVersion=$(yq -r ".spec.version" "${CSV}")
  echo  "[INFO] current nightly ${platform} version: ${currentNightlyVersion}"

  getNightlyVersionIncrementPart "${currentNightlyVersion}"

  PACKAGE_NAME="eclipse-che-preview-${platform}"

  CLUSTER_SERVICE_VERSION=$(getCurrentStableVersion "${platform}")
  STABLE_PACKAGE_VERSION=$(echo "${CLUSTER_SERVICE_VERSION}" | sed -e "s/${PACKAGE_NAME}.v//")

  parseStableVersion
  STABLE_MINOR_VERSION=$((STABLE_MINOR_VERSION+1))
  newVersion="${STABLE_MAJOR_VERSION}.${STABLE_MINOR_VERSION}.0-$((incrementPart+1)).nightly"

  echo "[INFO] Set up nightly ${platform} version: ${newVersion}"
  yq -rY "(.spec.version) = \"${newVersion}\" | (.metadata.name) = \"eclipse-che-preview-${platform}.v${newVersion}\"" "${CSV}" > "${CSV}.old"
  mv "${CSV}.old" "${CSV}"
}

getNightlyVersionIncrementPart() {
  nightlyVersion="${1}"

  versionWithoutNightly="${nightlyVersion%.nightly}"

  version="${versionWithoutNightly%-*}"

  incrementPart="${versionWithoutNightly#*-}"

  echo "${incrementPart}"
}

parseStableVersion() {
  local majorAndMinor=${STABLE_PACKAGE_VERSION%.*}
  STABLE_MINOR_VERSION=${majorAndMinor#*.}
  STABLE_MAJOR_VERSION=${majorAndMinor%.*}

  export STABLE_MAJOR_VERSION
  export STABLE_MINOR_VERSION
}
