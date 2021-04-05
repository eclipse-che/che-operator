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

# This script updates deploy/crds/org_v1_che_cr.yaml and
# deploy/crds/org_v1_che_crd.yaml files when `che_types.go` is changed.

set -e

init() {
  if [ -z "${BASE_DIR}" ]; then
    BASE_DIR=$(dirname $(readlink -f "${BASH_SOURCE[0]}"))
  fi
  OPERATOR_DIR="$(dirname "${BASE_DIR}")"
}

check() {
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
    exit 0
  fi 
}

updateFiles() {
  pushd "${BASE_DIR}"/.. || true
  "${OPERATOR_SDK_BINARY}" generate k8s
  "${OPERATOR_SDK_BINARY}" generate crds
  popd
}

removeRequired() {
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

      echo  "$line" >> $2
  done < "$1"
}

addLicenseHeader() {
  echo $1_license

cat << EOF > $1_license
$2
$2  Copyright (c) 2012-2020 Red Hat, Inc.
$2    This program and the accompanying materials are made
$2    available under the terms of the Eclipse Public License 2.0
$2    which is available at https://www.eclipse.org/legal/epl-2.0/
$2
$2  SPDX-License-Identifier: EPL-2.0
$2
$2  Contributors:
$2    Red Hat, Inc. - initial API and implementation
EOF

cat $1 >> $1_license
mv $1_license $1
}

init
check
updateFiles

rm "$BASE_DIR/../deploy/crds/org_v1_che_crd.yaml"
removeRequired "$BASE_DIR/../deploy/crds/org.eclipse.che_checlusters_crd.yaml" "$BASE_DIR/../deploy/crds/org_v1_che_crd.yaml"
rm "$BASE_DIR/../deploy/crds/org.eclipse.che_checlusters_crd.yaml"
addLicenseHeader "$BASE_DIR/../deploy/crds/org_v1_che_crd.yaml" "#"
