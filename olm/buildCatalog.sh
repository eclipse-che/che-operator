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

set -e
set -x

export OPERATOR_REPO="${GITHUB_WORKSPACE}"

if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$SCRIPT")")
fi
source "${OPERATOR_REPO}"/olm/olm.sh

init() {
  FORCE="false"
  unset CHANNEL
  unset PLATFORM
  unset CATALOG_IMAGE
  unset OPERATOR_IMAGE

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--channel'|'-c') CHANNEL="$2"; shift 1;;
      '--platform'|'-p') PLATFORM="$2"; shift 1;;
      '--catalog-image'|'-i') CATALOG_IMAGE="$2"; shift 1;;
      '--operator-image'|'-o') OPERATOR_IMAGE="$2"; shift 1;;
      '--force'|'-f') FORCE="true";;
      '--help'|'-h') usage; exit;;
    esac
    shift 1
  done

  if [[ ! ${CHANNEL} ]] || [[ ! ${PLATFORM} ]] || [[ ! ${CATALOG_IMAGE} ]]; then usage; exit 1; fi

  BUNDLE_DIR=$(getBundlePath "${PLATFORM}" "${CHANNEL}")
  OPM_BUNDLE_MANIFESTS_DIR="${BUNDLE_DIR}/manifests"
  CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"
  CSV_NAME=$(yq -r ".metadata.name" "${CSV}")
  CSV_VERSION=$(yq -r ".spec.version" "${CSV}")

  IMAGE_REGISTRY_HOST=$(echo ${CATALOG_IMAGE} | cut -d '/' -f1)
  IMAGE_REGISTRY_USER_NAME=$(echo ${CATALOG_IMAGE} | cut -d '/' -f2)
  BUNDLE_IMAGE="${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${PLATFORM}-opm-bundles:${CSV_VERSION}"

  echo "[INFO] CSV: ${CSV}"
  echo "[INFO] CSV name: ${CSV_NAME}"
  echo "[INFO] CSV version: ${CSV_VERSION}"
  echo "[INFO] Bundle image: ${BUNDLE_IMAGE}"

  if [[ ! -z ${OPERATOR_IMAGE} ]]; then
    # set a given operator image into CSV before build
    sed -e "s|image:.*|image: ${OPERATOR_IMAGE}|" -i "${CSV}"
  fi
}

usage () {
	echo "Usage:   $0 -p (openshift|kubernetes) -c (next|next-all-namespaces|stable|tech-preview-all-namespaces) -i CATALOG_IMAGE [-f]"
	echo "Example: $0 -p openshift -c next -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:next -f"
}

buildBundle() {
  if [[ $(isBundleImageExists) == 0 ]] || [[ "${FORCE}" == "true" ]]; then
    echo "[INFO] Build bundle image"
    buildBundleImage "${PLATFORM}" "${BUNDLE_IMAGE}" "${CHANNEL}" "docker"
  else
    echo "[INFO] Bundle image already exists. Use '--force' flag to force build."
  fi
}

buildCatalog () {
  if [ $(isCatalogExists) == 0 ]; then
    echo "[INFO] Bundle a new catalog"
    buildCatalogImage "${CATALOG_IMAGE}" "${BUNDLE_IMAGE}" "docker" "${FORCE}"
  else
    if [[ $(isBundleExistsInCatalog) == 0 ]]; then
      echo "[INFO] Add a new bundle ${CSV_NAME} to the catalog"
      buildCatalogImage "${CATALOG_IMAGE}" "${BUNDLE_IMAGE}" "docker" "${FORCE}" "${CATALOG_IMAGE}"
    else
      echo "[INFO] Bundle ${CSV_NAME} already exists in the catalog"
    fi
  fi
}

isBundleExistsInCatalog() {
  local BUNDLE_NAME=$(docker run --entrypoint sh ${CATALOG_IMAGE} -c "apk add sqlite && sqlite3 /database/index.db 'SELECT head_operatorbundle_name FROM channel WHERE name = \"${CHANNEL}\" and head_operatorbundle_name = \"${CSV_NAME}\"'" | tail -n1 | tr -d '\r')

  # docker run produce more output then a single line
  # so, it is needed to check if the last line is actually a given bunle name
  if [[ ${BUNDLE_NAME} == ${CSV_NAME} ]]; then echo 1; else echo 0; fi
}

isBundleImageExists() {
  skopeo inspect docker://${BUNDLE_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CSV_VERSION}\")" | wc -l
}

isCatalogExists() {
  CATALOG_TAG=$(echo $CATALOG_IMAGE | rev | cut -d ':' -f1 | rev)
  skopeo inspect docker://${CATALOG_IMAGE} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${CATALOG_TAG}\")" | wc -l
}

init $@
installOPM
buildBundle
buildCatalog

echo "[INFO] Done"
