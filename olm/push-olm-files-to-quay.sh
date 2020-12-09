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

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)

for platform in 'kubernetes' 'openshift'
do
  packageName="eclipse-che-preview-${platform}"

  if [ "${APPLICATION_REGISTRY}" == "" ]; then
    quayNamespace="eclipse-che-operator-${platform}"
  else
    quayNamespace="${APPLICATION_REGISTRY}"
  fi

  echo
  echo "## Pushing the OperatorHub package '${packageName}' for platform '${platform}' to the Quay.io '${quayNamespace}' organization"

  packageBaseFolderPath="${BASE_DIR}/${packageName}"
  cd "${packageBaseFolderPath}"

  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  flattenFolderPath="${packageBaseFolderPath}/generated/flatten"

  echo "   - Flatten package to temporary folder: ${flattenFolderPath}"

  rm -Rf "${flattenFolderPath}"
  mkdir -p "${flattenFolderPath}"
  operator-courier flatten "${packageFolderPath}" generated/flatten

  lastGitCommit=$(git log -n 1 --format="%h" -- .)
  applicationVersion="9.9.$(date +%s)+${lastGitCommit}"
  echo "   - Push flattened files to Quay.io namespace '${quayNamespace}' as version ${applicationVersion}"
  case ${platform} in
  "kubernetes")
    QUAY_USERNAME_PLATFORM_VAR="QUAY_USERNAME_K8S"
    QUAY_PASSWORD_PLATFORM_VAR="QUAY_PASSWORD_K8S"
    QUAY_ECLIPSE_CHE_USERNAME=${QUAY_USERNAME_K8S:-$QUAY_ECLIPSE_CHE_USERNAME}
    QUAY_ECLIPSE_CHE_PASSWORD=${QUAY_PASSWORD_K8S:-$QUAY_ECLIPSE_CHE_PASSWORD}
    ;;
  "openshift")
    QUAY_USERNAME_PLATFORM_VAR="QUAY_USERNAME_OS"
    QUAY_PASSWORD_PLATFORM_VAR="QUAY_PASSWORD_OS"
    QUAY_ECLIPSE_CHE_USERNAME=${QUAY_USERNAME_OS:-$QUAY_ECLIPSE_CHE_USERNAME}
    QUAY_ECLIPSE_CHE_PASSWORD=${QUAY_PASSWORD_OS:-$QUAY_ECLIPSE_CHE_PASSWORD}
    ;;
  esac
  if [ -z "${QUAY_ECLIPSE_CHE_USERNAME}" ] || [ -z "${QUAY_ECLIPSE_CHE_PASSWORD}" ]
  then
    echo "[ERROR] Must set ${QUAY_USERNAME_PLATFORM_VAR} and ${QUAY_PASSWORD_PLATFORM_VAR} environment variables"
    echo "[ERROR] with a user that has write access to the following Quay.io application namespace: ${quayNamespace}"
    echo "[ERROR] or QUAY_ECLIPSE_CHE_USERNAME and QUAY_ECLIPSE_CHE_PASSWORD if the same user can access both "
    echo "[ERROR] application namespaces 'eclipse-che-operator-kubernetes' and 'eclipse-che-operator-openshift'"
    exit 1
  fi
  # echo "[DEBUG] Authenticating with: QUAY_ECLIPSE_CHE_USERNAME = ${QUAY_ECLIPSE_CHE_USERNAME}"
  AUTH_TOKEN=$(curl -sH "Content-Type: application/json" -XPOST https://quay.io/cnr/api/v1/users/login -d '
{
    "user": {
        "username": "'"${QUAY_ECLIPSE_CHE_USERNAME}"'",
        "password": "'"${QUAY_ECLIPSE_CHE_PASSWORD}"'"
    }
}' | jq -r '.token')
  # if [[ ${AUTH_TOKEN} ]]; then echo "[DEBUG] Got token"; fi

  # move all diff files away so we don't get warnings about invalid file names
  find . -name "*.yaml.diff" -exec rm -f {} \; || true

  # push new applications to quay.io/application/eclipse-che-operator-*
  operator-courier push generated/flatten "${quayNamespace}" "${packageName}" "${applicationVersion}" "${AUTH_TOKEN}"

  # now put them back
  git checkout . || true
done
cd "${CURRENT_DIR}"
