#!/bin/bash
#
# Copyright (c) 2019 Red Hat, Inc.
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

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
TAG=$1
source ${BASE_DIR}/check-yq.sh

for platform in 'kubernetes' 'openshift'
do
  packageName=eclipse-che-preview-${platform}

  echo "[INFO] Updating OperatorHub package '${packageName}' for platform '${platform}'"
  packageBaseFolderPath=${BASE_DIR}/${packageName}

  cd "${packageBaseFolderPath}"
  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  lastPackageVersion=$(yq -r '.channels[] | select(.name == "nightly") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")

  echo "[INFO] Last package version: ${lastPackageVersion}"
  newNightlyPackageVersion="9.9.9-nightly.$(date +%s)"

  PREV_CRD="${packageFolderPath}/${lastPackageVersion}/eclipse-che-preview-${platform}.crd.yaml"
  PREV_CSV="${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml"
  NEW_CSV="${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml"
  NEW_CRD="${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}.crd.yaml"

  echo "[INFO] will create a new version: ${newNightlyPackageVersion}"
  ./build-roles.sh

  echo "[INFO] Updating new package version with roles defined in: ${role}"
  operator-sdk olm-catalog gen-csv --csv-version "${newNightlyPackageVersion}" --from-version="${lastPackageVersion}" 2>&1 | sed -e 's/^/      /'
  containerImage=$(sed -n 's|^ *image: *\([^ ]*/che-operator:[^ ]*\) *|\1|p' ${NEW_CSV})
  createdAt=$(date -u +%FT%TZ)

  echo "[INFO] Updating new package version fields:"
  echo "[INFO]        - containerImage => ${containerImage}"
  echo "[INFO]        - createdAt => ${createdAt}"
  sed \
  -e "s|containerImage:.*$|containerImage: ${containerImage}|" \
  -e "s/createdAt:.*$/createdAt: \"${createdAt}\"/" ${NEW_CSV} > ${NEW_CSV}".new"
  mv ${NEW_CSV}".new" ${NEW_CSV}

  echo "[INFO] Copying the CRD file"
  cp "${BASE_DIR}/../deploy/crds/org_v1_che_crd.yaml" ${NEW_CRD}

  echo "[INFO] Updating the 'nightly' channel with new version in the package descriptor: ${packageFilePath}"
  sed -e "s/${lastPackageVersion}/${newNightlyPackageVersion}/" "${packageFilePath}" > "${packageFilePath}.new"
  mv "${packageFilePath}.new" "${packageFilePath}"

  if [[ ! -z "$TAG" ]]; then
    echo "[INFO] Set tags in nighlty OLM files"
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

  diff -u ${PREV_CRD} ${NEW_CRD} > ${NEW_CRD}".diff" || true
  diff -u ${PREV_CSV} ${NEW_CSV} > ${NEW_CSV}".diff" || true
done
cd "${CURRENT_DIR}"
