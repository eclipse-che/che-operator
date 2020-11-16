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
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
}

check() {
  local operatorVersion=$(operator-sdk version)
  [[ $operatorVersion =~ .*v0.17.1.* ]] || { echo "operator-sdk v0.17.1 is required"; exit 1; }
}

updateFiles() {
  cd $BASE_DIR/..
  operator-sdk generate k8s
  operator-sdk generate crds
  cd $BASE_DIR
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
