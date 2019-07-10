#!/bin/bash
#
# Copyright (c) 2012-2018 Red Hat, Inc.
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
  packageName=eclipse-che-test-${platform}
  quayNamespace=eclipse-che-operator-${platform}
  echo
  echo "## Pushing the OperatorHub package '${packageName}' for platform '${platform}' to the Quay.io '${quayNamespace}' organization"

  packageBaseFolderPath=${BASE_DIR}/${packageName}
  cd ${packageBaseFolderPath}

  packageFolderPath=${packageBaseFolderPath}/deploy/olm-catalog/${packageName}
  flattenFolderPath=${packageBaseFolderPath}/generated/flatten

  echo "   - Flatten package to temporary folder: ${flattenFolderPath}"
  
  rm -Rf ${flattenFolderPath}
  mkdir -p ${flattenFolderPath}
  operator-courier flatten deploy/olm-catalog/${packageName} generated/flatten

  lastGitCommit=$(git log -n 1 --format="%h" -- .)
  applicationVersion="9.9.$(date +%s)+${lastGitCommit}"
  echo "   - Push flattened files to Quay.io namespace '${quayNamespace}' as version ${applicationVersion}"
  case ${platform} in
  "kubernetes")
    QUAY_USERNAME_PLATFORM_VAR="QUAY_USERNAME_K8S"
    QUAY_PASSWORD_PLATFORM_VAR="QUAY_PASSWORD_K8S"
    QUAY_USERNAME=${QUAY_USERNAME_K8S:-$QUAY_USERNAME}
    QUAY_PASSWORD=${QUAY_PASSWORD_K8S:-$QUAY_PASSWORD}
    ;;
  "openshift")
    QUAY_USERNAME_PLATFORM_VAR="QUAY_USERNAME_OS"
    QUAY_PASSWORD_PLATFORM_VAR="QUAY_PASSWORD_OS"
    QUAY_USERNAME=${QUAY_USERNAME_OS:-$QUAY_USERNAME}
    QUAY_PASSWORD=${QUAY_PASSWORD_OS:-$QUAY_PASSWORD}
    ;;
  esac
  if [ -z "${QUAY_USERNAME}" -o -z "${QUAY_PASSWORD}" ]
  then
    echo "#### ERROR: "
    echo "You should have set ${QUAY_USERNAME_PLATFORM_VAR} and ${QUAY_PASSWORD_PLATFORM_VAR} environment variables"
    echo "with a user that has write access to the following Quay.io namespace: ${quayNamespace}"
    echo "or QUAY_USERNAME and QUAY_PASSWORD if the same user can access both namespaces 'eclipse-che-operator-kubernetes' and 'eclipse-che-operator-openshift'"
    exit 1
  fi
  AUTH_TOKEN=$(curl -sH "Content-Type: application/json" -XPOST https://quay.io/cnr/api/v1/users/login -d '
{
    "user": {
        "username": "'"${QUAY_USERNAME}"'",
        "password": "'"${QUAY_PASSWORD}"'"
    }
}' | jq -r '.token')

  operator-courier push generated/flatten ${quayNamespace} ${packageName} "${applicationVersion}" "$AUTH_TOKEN"
done
cd ${CURRENT_DIR}