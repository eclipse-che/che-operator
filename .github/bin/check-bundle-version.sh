#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
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

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
  ROOT_PROJECT_DIR=$(dirname "$(dirname "${BASE_DIR}")")
fi

CSV_KUBERNETES_NEW="bundle/next/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml"
CSV_KUBERNETES_CURRENT=https://raw.githubusercontent.com/eclipse-che/che-operator/main/deploy/olm-catalog/next/eclipse-che-preview-kubernetes/manifests/che-operator.clusterserviceversion.yaml

CSV_OPENSHIFT_NEW="bundle/next/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"
CSV_OPENSHIFT_CURRENT=https://raw.githubusercontent.com/eclipse-che/che-operator/main/deploy/olm-catalog/next/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml

checkNextBundleVersions() {
  git remote add operator https://github.com/eclipse-che/che-operator.git
  git fetch operator -q
  git fetch origin -q

  changedFiles=(
    $(git diff --name-only refs/remotes/operator/${GITHUB_BASE_REF})
  )

  for file in "${changedFiles[@]}"
  do
    echo "[INFO] Changed file: $file"
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
    echo "[ERROR] New next bundle version is less than the current one."
    echo "[ERROR] Please update next bundle with script 'make update-resources -s'"
    exit 1
  fi
}

convertVersionToNumber() {
  version=$1                                    # 7.28.1-130.next
  versionWithoutNext="${version%.next}"         # 7.28.1-130
  version="${versionWithoutNext%-*}"            # 7.28.1
  incrementPart="${versionWithoutNext#*-}"      # 130
  major=$(echo $version | cut  -d '.' -f 1)     # 7
  minor=$(echo $version | cut  -d '.' -f 2)     # 28
  bugfix=$(echo $version | cut  -d '.' -f 3)    # 1

  # 702810130
  echo $((major * 100000000 + minor * 100000 + bugfix * 10000 + incrementPart))
}

checkNextBundleVersions

echo "[INFO] Done."
