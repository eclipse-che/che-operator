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

REGEX="^([0-9]+)\\.([0-9]+)\\.([0-9]+)(\\-[0-9a-z-]+(\\.[0-9a-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

BASE_DIR=$(dirname $(dirname $(readlink -f "${BASH_SOURCE[0]}")))/olm
source ${BASE_DIR}/check-yq.sh
GO_VERSION_FILE=$(readlink -f "${BASE_DIR}/../version/version.go")

if [[ "$1" =~ $REGEX ]]
then
  RELEASE="$1"
else
  echo "You should provide the new release as the first parameter"
  echo "and it should be semver-compatible with optional *lower-case* pre-release part"
  exit 1
fi

for platform in 'kubernetes' 'openshift'
do
  # todo
  PACKAGE_VERSION="stable"
  export PACKAGE_VERSION
  source ${BASE_DIR}/olm.sh "${platform}"

  echo "[INFO] Creating release '${RELEASE}' for platform '${platform}'"

  NIGHTLY_BUNDLE_PATH=$(getBundlePath "${platform}" "nightly")
  LAST_NIGHTLY_CSV="${NIGHTLY_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  LAST_NIGHTLY_CRD="${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"

  STABLE_BUNDLE_PATH=$(getBundlePath "${platform}" "stable")
  LAST_STABLE_CSV="${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"

  lastPackageNightlyVersion=$(yq -r ".spec.version" "${LAST_NIGHTLY_CSV}")
  if [ -f "${LAST_STABLE_CSV}" ];then
    lastPackagePreReleaseVersion=$(yq -r ".spec.version" "${LAST_STABLE_CSV}")
  else
    lastPackagePreReleaseVersion=$(grep -o '[0-9]*\.[0-9]*\.[0-9]*' < "${GO_VERSION_FILE}")
  fi

  echo "[INFO] Last package nightly version: ${lastPackageNightlyVersion}"

  if [ "${lastPackagePreReleaseVersion}" == "${RELEASE}" ]
  then
    echo "[ERROR] Release ${RELEASE} already exists in the package !"
    echo "[ERROR] You should first remove it"
    exit 1
  fi

  echo "[INFO] Will create release '${RELEASE}' from nightly version ${lastPackageNightlyVersion}'"

  RELEASE_CSV="${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  RELEASE_CRD="${STABLE_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"

  mkdir -p "${STABLE_BUNDLE_PATH}/manifests" "${STABLE_BUNDLE_PATH}/generated" "${STABLE_BUNDLE_PATH}/metadata"
  if [[ -f "${RELEASE_CSV}" ]] && [[ -f "${RELEASE_CRD}" ]]; then
    cp -rf "${RELEASE_CSV}" "${RELEASE_CRD}" "${STABLE_BUNDLE_PATH}/generated/"
    PRE_RELEASE_CSV="${STABLE_BUNDLE_PATH}/generated/che-operator.clusterserviceversion.yaml"
    PRE_RELEASE_CRD="${STABLE_BUNDLE_PATH}/generated/org_v1_che_crd.yaml"
  fi

  sed \
  -e 's/imagePullPolicy: *Always/imagePullPolicy: IfNotPresent/' \
  -e 's/"cheImageTag": *"nightly"/"cheImageTag": ""/' \
  -e 's|"identityProviderImage": *"quay.io/eclipse/che-keycloak:nightly"|"identityProviderImage": ""|' \
  -e 's|"devfileRegistryImage": *"quay.io/eclipse/che-devfile-registry:nightly"|"devfileRegistryImage": ""|' \
  -e 's|"pluginRegistryImage": *"quay.io/eclipse/che-plugin-registry:nightly"|"pluginRegistryImage": ""|' \
  -e "s/^  version: ${lastPackageNightlyVersion}/  version: ${RELEASE}/" \
  -e "s/: nightly/: ${RELEASE}/" \
  -e "s/:nightly/:${RELEASE}/" \
  -e "s/${lastPackageNightlyVersion}/${RELEASE}/" \
  -e "s/createdAt:.*$/createdAt: \"$(date -u +%FT%TZ)\"/" "${LAST_NIGHTLY_CSV}" > "${RELEASE_CSV}"

  cp "${LAST_NIGHTLY_CRD}" "${RELEASE_CRD}"
  cp -rf "${NIGHTLY_BUNDLE_PATH}/bundle.Dockerfile" "${STABLE_BUNDLE_PATH}"
  cp -rf "${NIGHTLY_BUNDLE_PATH}/metadata" "${STABLE_BUNDLE_PATH}"

  ANNOTATION_METADATA_YAML="${STABLE_BUNDLE_PATH}/metadata/annotations.yaml"
  sed \
  -e 's/operators.operatorframework.io.bundle.channels.v1: *nightly/operators.operatorframework.io.bundle.channels.v1: stable/' \
  -e 's/operators.operatorframework.io.bundle.channel.default.v1: *nightly/operators.operatorframework.io.bundle.channel.default.v1: stable/' \
  -i "${ANNOTATION_METADATA_YAML}"

  BUNDLE_DOCKERFILE="${STABLE_BUNDLE_PATH}/bundle.Dockerfile"
  sed \
  -e 's/LABEL operators.operatorframework.io.bundle.channels.v1=nightly/LABEL operators.operatorframework.io.bundle.channels.v1=stable/' \
  -e 's/LABEL operators.operatorframework.io.bundle.channel.default.v1=nightly/LABEL operators.operatorframework.io.bundle.channel.default.v1=stable/' \
  -i "${BUNDLE_DOCKERFILE}"

  sed -e "s|Version = \".*\"|Version = \"${RELEASE}\"|" -i "${GO_VERSION_FILE}"

  # PLATFORM_DIR=$(pwd)

  # cd $CURRENT_DIR
  # source ${BASE_DIR}/addDigests.sh -w ${BASE_DIR} \
  #               -r "eclipse-che-preview-${platform}.*\.v${RELEASE}.*yaml" \
  #               -t ${RELEASE}

  # cd $PLATFORM_DIR

  if [[ -n "${PRE_RELEASE_CSV}" ]] && [[ -n "${PRE_RELEASE_CRD}" ]]; then 
    diff -u "${PRE_RELEASE_CSV}" "${RELEASE_CSV}" > "${RELEASE_CSV}.diff" || true
    diff -u "${PRE_RELEASE_CRD}" "${RELEASE_CRD}" > "${RELEASE_CRD}.diff" || true
  fi
done

echo "[INFO] Release bundles successfully created."
