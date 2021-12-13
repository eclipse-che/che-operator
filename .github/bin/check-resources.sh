#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# Checks if repository resources are up to date:
# - CRDs
# - next olm bundle
# - Dockerfile & operator.yaml
# - DW resources
# - Helm charts

set -e

ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
if [ -z "${ROOT_PROJECT_DIR}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  ROOT_PROJECT_DIR=$(dirname $(dirname $(dirname ${SCRIPT})))
fi

installOperatorSDK() {
  OPERATOR_SDK_BINARY=$(command -v operator-sdk) || true
  if [[ ! -x "${OPERATOR_SDK_BINARY}" ]]; then
    OPERATOR_SDK_TEMP_DIR="$(mktemp -q -d -t "OPERATOR_SDK_XXXXXX" 2>/dev/null || mktemp -q -d)"

    pushd "${ROOT_PROJECT_DIR}" || exit
    make download-operator-sdk OP_SDK_DIR="${OPERATOR_SDK_TEMP_DIR}"
    export OPERATOR_SDK_BINARY="${OPERATOR_SDK_TEMP_DIR}/operator-sdk"
    popd || exit
  fi
}

updateResources() {
  echo "[INFO] Update resources with skipping version incrementation and timestamp..."

  pushd "${ROOT_PROJECT_DIR}" || exit
  make update-resources NO_DATE_UPDATE="true" NO_INCREMENT="true" -s
  popd || exit
}

# check_che_types function check first if api/v1/checluster_types.go file suffer modifications and
# in case of modification should exist also modifications in config/crd/bases/* folder.
checkCRDs() {
    echo "[INFO] Checking CRDs"

    # files to check
    local checluster_CRD_V1="config/crd/bases/org_v1_che_crd.yaml"
    local chebackupserverconfiguration_CRD_V1="config/crd/bases/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
    local checlusterbackup_CRD_V1="config/crd/bases/org.eclipse.che_checlusterbackups_crd.yaml"
    local checlusterrestore_CRD_V1="config/crd/bases/org.eclipse.che_checlusterrestores_crd.yaml"

    local checluster_CRD_V1BETA1="config/crd/bases/org_v1_che_crd-v1beta1.yaml"
    local chebackupserverconfiguration_CRD_V1BETA1="config/crd/bases/org.eclipse.che_chebackupserverconfigurations_crd-v1beta1.yaml"
    local checlusterbackup_CRD_V1BETA1="config/crd/bases/org.eclipse.che_checlusterbackups_crd-v1beta1.yaml"
    local checlusterrestore_CRD_V1BETA1="config/crd/bases/org.eclipse.che_checlusterrestores_crd-v1beta1.yaml"

    changedFiles=($(cd ${ROOT_PROJECT_DIR}; git diff --name-only))
    # Check if there are any difference in the crds. If yes, then fail check.
    if [[ " ${changedFiles[*]} " =~ $checluster_CRD_V1 ]] || [[ " ${changedFiles[*]} " =~ $checluster_CRD_V1BETA1 ]] || \
       [[ " ${changedFiles[*]} " =~ $chebackupserverconfiguration_CRD_V1 ]] || [[ " ${changedFiles[*]} " =~ $chebackupserverconfiguration_CRD_V1BETA1 ]] || \
       [[ " ${changedFiles[*]} " =~ $checlusterbackup_CRD_V1 ]] || [[ " ${changedFiles[*]} " =~ $checlusterbackup_CRD_V1BETA1 ]] || \
       [[ " ${changedFiles[*]} " =~ $checlusterrestore_CRD_V1 ]] || [[ " ${changedFiles[*]} " =~ $checlusterrestore_CRD_V1BETA1 ]]
    then
        echo "[ERROR] CRD file is not up to date: ${BASH_REMATCH}"
        echo "[ERROR] Run 'make update-resources -s' to regenerate CRD files."
        exit 1
    else
        echo "[INFO] CRDs files are up to date."
    fi
}

checkNextOlmBundle() {
  # files to check
  local CSV_OPENSHIFT="bundle/next/eclipse-che-preview-openshift/manifests"

  changedFiles=($(cd ${ROOT_PROJECT_DIR}; git diff --name-only))
  if [[ " ${changedFiles[*]} " =~ $CSV_OPENSHIFT ]]; then
    echo "[ERROR] Nighlty bundle is not up to date: ${BASH_REMATCH}"
    echo "[ERROR] Run 'make update-resources -s' to regenerate next bundle files."
    exit 1
  else
    echo "[INFO] Next bundles are up to date."
  fi
}

checkDockerfile() {
  # files to check
  local Dockerfile="Dockerfile"

  changedFiles=($(cd ${ROOT_PROJECT_DIR}; git diff --name-only))
  if [[ " ${changedFiles[*]} " =~ $Dockerfile ]]; then
    echo "[ERROR] Dockerfile is not up to date"
    echo "[ERROR] Run 'make update-resources -s' to update Dockerfile"
    exit 1
  else
    echo "[INFO] Dockerfile is up to date."
  fi
}

checkOperatorYaml() {
  # files to check
  local managerYaml="config/manager/manager.yaml"

  changedFiles=($(cd ${ROOT_PROJECT_DIR}; git diff --name-only))
  if [[ " ${changedFiles[*]} " =~ $managerYaml ]]; then
    echo "[ERROR] $managerYaml is not up to date"
    echo "[ERROR] Run 'make update-resources -s' to update $managerYaml"
    exit 1
  else
    echo "[INFO] $managerYaml is up to date."
  fi
}

checkRoles() {
  # files to check
  local RoleYaml="config/rbac/role.yaml"
  local ClusterRoleYaml="config/rbac/cluster_role.yaml"

  changedFiles=(
    $(git diff --name-only)
  )
  if [[ " ${changedFiles[*]} " =~ $RoleYaml ]] || [[ " ${changedFiles[*]} " =~ $ClusterRoleYaml ]]; then
    echo "[ERROR] Roles are not up to date: ${BASH_REMATCH}"
    echo "[ERROR] Run 'make update-resources -s' to update them."
    exit 1
  else
    echo "[INFO] Roles are up to date."
  fi
}

checkHelmCharts() {
  changedFiles=(
    $(git diff --name-only)
  )
  if [[ " ${changedFiles[*]} " =~ helmcharts ]]; then
    echo "[ERROR] Helm Charts are not up to date"
    echo "[ERROR] Run 'make update-resources -s' to update them."
    exit 1
  else
    echo "[INFO] Helm Charts are up to date."
  fi
}

installOperatorSDK

pushd "${ROOT_PROJECT_DIR}" || true

updateResources
checkCRDs
checkRoles
checkNextOlmBundle
checkDockerfile
checkOperatorYaml
checkHelmCharts

popd || true

echo "[INFO] Done."
