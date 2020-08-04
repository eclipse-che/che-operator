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
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
fi

if [ -z "${OPERATOR_SDK_BINARY}" ]; then
  OPERATOR_SDK_BINARY=$(command -v operator-sdk)
  if [[ ! -x "${OPERATOR_SDK_BINARY}" ]]; then
    echo "[ERROR] operator-sdk is not installed."
    exit 1
  fi
fi

# BASE_DIR=$(cd "$(dirname "$0")"; pwd)

ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")
TAG=$1
source ${BASE_DIR}/check-yq.sh

# TODO check operator sdk version...
for platform in 'kubernetes' 'openshift'
do
  echo "[INFO] Updating OperatorHub bundle for platform '${platform}' for platform '${platform}'"

  pushd "${ROOT_PROJECT_DIR}" || true

  olmCatalog=${ROOT_PROJECT_DIR}/deploy/olm-catalog
  operatorFolder=${olmCatalog}/che-operator
  bundleFolder=${operatorFolder}/eclipse-che-preview-${platform}

  # todo, hardcoded...
  newNightlyBundleVersion="7.16.2-0.nightly"
  bundleCSVName="che-operator.clusterserviceversion.yaml"
  NEW_CSV=${bundleFolder}/manifests/${bundleCSVName}
  echo "[INFO] Will create new nightly bundle version: ${newNightlyBundleVersion}"

  "${bundleFolder}"/build-roles.sh

  packageManifestFolderPath=${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/${newNightlyBundleVersion}
  packageManifestCSVPath=${packageManifestFolderPath}/che-operator.v${newNightlyBundleVersion}.clusterserviceversion.yaml

  mkdir -p "${packageManifestFolderPath}"
  cp -rf "${NEW_CSV}" "${packageManifestCSVPath}"
  cp -rf "${bundleFolder}/csv-config.yaml" "${olmCatalog}"

  echo "[INFO] Updating new package version..."
  "${OPERATOR_SDK_BINARY}" olm-catalog gen-csv --csv-version "${newNightlyBundleVersion}" 2>&1 | sed -e 's/^/      /'

  cp -rf "${packageManifestCSVPath}" "${NEW_CSV}"

  rm -rf "${packageManifestFolderPath}" "${packageManifestCSVPath}" "${operatorFolder}/che-operator.package.yaml" "${olmCatalog}/csv-config.yaml"

  containerImage=$(sed -n 's|^ *image: *\([^ ]*/che-operator:[^ ]*\) *|\1|p' ${NEW_CSV})
  createdAt=$(date -u +%FT%TZ)

  echo "[INFO] Updating new package version fields:"
  echo "[INFO]        - containerImage => ${containerImage}"
  echo "[INFO]        - createdAt => ${createdAt}"
  sed \
  -e "s|containerImage:.*$|containerImage: ${containerImage}|" \
  -e "s/createdAt:.*$/createdAt: \"${createdAt}\"/" \
  "${NEW_CSV}" > "${NEW_CSV}.new"
  mv "${NEW_CSV}.new" "${NEW_CSV}"

    # -e "s/createdAt:.*$/createdAt: \"${createdAt}\"/" \
 
  cp -rf "${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd.yaml" "${bundleFolder}/manifests"
  echo "Done for ${platform}"

  if [[ -n "$TAG" ]]; then
    echo "[INFO] Set tags in nightly OLM files"
    sed -i 's/'$RELEASE'/'$TAG'/g' ${NEW_CSV}
  fi

  if [[ $platform == "openshift" ]]; then
    # Removes che-tls-secret-creator
    index=0
    while [[ $index -le 30 ]]
    do
      if [[ $(cat ${NEW_CSV} | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env['$index'].name') == "RELATED_IMAGE_che_tls_secrets_creation_job" ]]; then
        yq -rYSi 'del(.spec.install.spec.deployments[0].spec.template.spec.containers[0].env['$index'])' ${NEW_CSV}
        break
      fi
      index=$((index+1))
    done
  fi

  popd || true
done
