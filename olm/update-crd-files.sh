#!/bin/bash
#
# Copyright (c) 2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

# Generated CRDs based on pkg/apis/org/v1/che_types.go:
# - deploy/crds/org_v1_che_crd.yaml
# - deploy/crds/org_v1_che_crd-v1beta1.yaml

set -e

init() {
  if [ -z "${BASE_DIR}" ]; then
    BASE_DIR=$(dirname $(readlink -f "${BASH_SOURCE[0]}"))
  fi
  if [ -z "${OPERATOR_DIR}" ]; then
    OPERATOR_DIR="$(dirname "${BASE_DIR}")"
  fi
}

checkOperatorSDKVersion() {
  if [ -z "${OPERATOR_SDK_BINARY}" ]; then
    OPERATOR_SDK_BINARY=$(command -v operator-sdk)
    if [[ ! -x "${OPERATOR_SDK_BINARY}" ]]; then
      echo "[ERROR] operator-sdk is not installed."
      exit 1
    fi
  fi

  local operatorVersion=$("${OPERATOR_SDK_BINARY}" version)
  REQUIRED_OPERATOR_SDK=$(yq -r ".\"operator-sdk\"" "${OPERATOR_DIR}/REQUIREMENTS")
  [[ $operatorVersion =~ .*${REQUIRED_OPERATOR_SDK}.* ]] || { echo "operator-sdk ${REQUIRED_OPERATOR_SDK} is required"; exit 1; }

  if [ -z "${GOROOT}" ]; then
    echo "[ERROR] set up '\$GOROOT' env variable to make operator-sdk working"
    exit 1
  fi
}

generateCRD() {
  version=$1
  pushd "${OPERATOR_DIR}" || true
  "${OPERATOR_SDK_BINARY}" generate k8s
  "${OPERATOR_SDK_BINARY}" generate crds --crd-version $version
  popd

  addLicenseHeader ${OPERATOR_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml

  if [[ $version == "v1" ]]; then
    mv ${OPERATOR_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml ${OPERATOR_DIR}/deploy/crds/org_v1_che_crd.yaml
    echo "[INFO] Generated CRD v1 ${OPERATOR_DIR}/deploy/crds/org_v1_che_crd.yaml"
  elif [[ $version == "v1beta1" ]]; then
    removeRequiredAttribute ${OPERATOR_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml
    mv ${OPERATOR_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml ${OPERATOR_DIR}/deploy/crds/org_v1_che_crd-v1beta1.yaml
    echo "[INFO] Generated CRD v1beta1 ${OPERATOR_DIR}/deploy/crds/org_v1_che_crd-v1beta1.yaml"
  fi
}

# Removes `required` attributes for fields to be compatible with OCP 3.11
removeRequiredAttribute() {
  REQUIRED=false
  while IFS= read -r line
  do
      if [[ $REQUIRED == true ]]; then
          if [[ $line == *"- "* ]]; then
              continue
          else
              REQUIRED=false
          fi
      fi

      if [[ $line == *"required:"* ]]; then
          REQUIRED=true
          continue
      fi

      echo  "$line" >> $1.tmp
  done < "$1"
  mv $1.tmp $1
}

addLicenseHeader() {
echo -e "#
#  Copyright (c) 2019-2021 Red Hat, Inc.
#    This program and the accompanying materials are made
#    available under the terms of the Eclipse Public License 2.0
#    which is available at https://www.eclipse.org/legal/epl-2.0/
#
#  SPDX-License-Identifier: EPL-2.0
#
#  Contributors:
#    Red Hat, Inc. - initial API and implementation
$(cat $1)" > $1
}

init
checkOperatorSDKVersion
generateCRD "v1"
generateCRD "v1beta1"
