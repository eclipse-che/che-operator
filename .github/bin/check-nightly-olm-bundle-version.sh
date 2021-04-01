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
set -x

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
  ROOT_PROJECT_DIR=$(dirname "$(dirname "${BASE_DIR}")")
fi

CSV_KUBERNETES_NEW="deploy/olm-catalog/nightly/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml"
CSV_KUBERNETES_CURRENT=https://raw.githubusercontent.com/eclipse-che/che-operator/master/deploy/olm-catalog/nightly/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml

CSV_OPENSHIFT_NEW="deploy/olm-catalog/nightly/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"
CSV_OPENSHIFT_CURRENT=https://raw.githubusercontent.com/eclipse-che/che-operator/master/deploy/olm-catalog/nightly/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml

checkNightlyBundleVersions() {
  export NO_DATE_UPDATE="true"
  export NO_INCREMENT="true"

  source "${ROOT_PROJECT_DIR}/olm/update-nightly-bundle.sh"

  IFS=$'\n' read -d '' -r -a changedFiles < <( git ls-files -m ) || true
  for file in "${changedFiles[@]}"
  do
    echo $file
    if [[ "${CSV_KUBERNETES_NEW}" == "${file}" ]]; then
      compareVersions ${ROOT_PROJECT_DIR}/$CSV_KUBERNETES_NEW $CSV_KUBERNETES_CURRENT
    elif [[ "${CSV_OPENSHIFT_NEW}" == "${file}" ]]; then
      compareVersions ${ROOT_PROJECT_DIR}/$CSV_OPENSHIFT_NEW $CSV_OPENSHIFT_CURRENT
    fi
  done
}

compareVersions() {
  CSV_VERSION_NEW=$(yq -r ".spec.version" $1)
  CSV_VERSION_CURRENT=$(curl -s $2 | yq -r ".spec.version")

  echo "[INFO] New version: $CSV_VERSION_NEW"
  echo "[INFO] Current version: $CSV_VERSION_CURRENT"

  VERSION_CURRENT_NUMBER=$(convertVersionToNumber $CSV_VERSION_CURRENT)
  VERSION_NEW_NUMBER=$(convertVersionToNumber $CSV_VERSION_NEW)

  if (( $VERSION_NEW_NUMBER <= $VERSION_CURRENT_NUMBER )); then
    echo "[ERROR] New nightly bundle version is less than the current one."
    echo "[ERROR] Please update nightly bundle with script 'olm/update-nightly-bundle.sh'"
    exit 1
  fi
}

convertVersionToNumber() {
  version=$1                                    # 7.28.1-130.nightly
  versionWithoutNightly="${version%.nightly}"   # 7.28.1-130
  version="${versionWithoutNightly%-*}"         # 7.28.1
  incrementPart="${versionWithoutNightly#*-}"   # 130
  major=$(echo $version | cut  -d '.' -f 1)     # 7
  minor=$(echo $version | cut  -d '.' -f 2)     # 28
  bugfix=$(echo $version | cut  -d '.' -f 3)    # 1

  # 702810130
  echo $((major * 100000000 + minor * 100000 + bugfix * 10000 + incrementPart))
}

checkNightlyBundleVersions

echo "[INFO] Done."
