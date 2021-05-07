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

OPERATOR_DIR=$(dirname $(dirname $(readlink -f "${BASH_SOURCE[0]}")))
BASE_DIR="${OPERATOR_DIR}/olm"
source ${BASE_DIR}/check-yq.sh

command -v pysemver >/dev/null 2>&1 || { echo "[ERROR] pysemver is not installed. Abort."; exit 1; }

export LAST_RELEASE_VERSION

setLatestReleasedVersion() {
  versions=$(curl \
  -H "Authorization: bearer ${GITHUB_TOKEN}" \
  -X POST -H "Content-Type: application/json" --data \
  '{"query": "{ repository(owner: \"eclipse\", name: \"che-operator\") { refs(refPrefix: \"refs/tags/\", last: 2, orderBy: {field: TAG_COMMIT_DATE, direction: ASC}) { edges { node { name } } } } }" } ' \
  https://api.github.com/graphql)

  LAST_RELEASE_VERSION=$(echo "${versions[@]}" | jq '.data.repository.refs.edges[1].node.name | sub("\""; "")' | tr -d '"')
}

downloadLatestReleasedBundleCRCRD() {
  mkdir -p "${STABLE_BUNDLE_PATH}/manifests" "${STABLE_BUNDLE_PATH}/generated/${platform}" "${STABLE_BUNDLE_PATH}/metadata"
  PRE_RELEASE_CSV="${STABLE_BUNDLE_PATH}/generated/${platform}/che-operator.clusterserviceversion.yaml"
  PRE_RELEASE_CRD="${STABLE_BUNDLE_PATH}/generated/${platform}/org_v1_che_crd.yaml"

  compareResult=$(pysemver compare "${LAST_RELEASE_VERSION}" "7.27.2")
  if [ "${compareResult}" == "1" ]; then
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/che-operator.clusterserviceversion.yaml" \
        -q -O "${PRE_RELEASE_CSV}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/org_v1_che_crd.yaml" \
        -q -O "${PRE_RELEASE_CRD}"
  else
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/olm/eclipse-che-preview-${platform}/deploy/olm-catalog/eclipse-che-preview-${platform}/${LAST_RELEASE_VERSION}/eclipse-che-preview-${platform}.v${LAST_RELEASE_VERSION}.clusterserviceversion.yaml" \
         -q -O "${PRE_RELEASE_CSV}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/olm/eclipse-che-preview-${platform}/deploy/olm-catalog/eclipse-che-preview-${platform}/${LAST_RELEASE_VERSION}/eclipse-che-preview-${platform}.crd.yaml" \
          -q -O "${PRE_RELEASE_CRD}"
  fi
}

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
  source ${BASE_DIR}/olm.sh

  echo "[INFO] Creating release '${RELEASE}' for platform '${platform}'"

  NIGHTLY_BUNDLE_PATH=$(getBundlePath "${platform}" "nightly")
  LAST_NIGHTLY_CSV="${NIGHTLY_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  LAST_NIGHTLY_CRD="${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
  lastPackageNightlyVersion=$(yq -r ".spec.version" "${LAST_NIGHTLY_CSV}")
  echo "[INFO] Last package nightly version: ${lastPackageNightlyVersion}"

  STABLE_BUNDLE_PATH=$(getBundlePath "${platform}" "stable")
  RELEASE_CSV="${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  RELEASE_CRD="${STABLE_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"

  setLatestReleasedVersion
  downloadLatestReleasedBundleCRCRD
  packageName=$(getPackageName "${platform}")

  echo "[INFO] Will create release '${RELEASE}' from nightly version ${lastPackageNightlyVersion}'"

  sed \
  -e 's/imagePullPolicy: *Always/imagePullPolicy: IfNotPresent/' \
  -e 's/"cheImageTag": *"nightly"/"cheImageTag": ""/' \
  -e 's|quay.io/eclipse/che-dashboard:next|quay.io/eclipse/che-dashboard:'${RELEASE}'|' \
  -e 's|"identityProviderImage": *"quay.io/eclipse/che-keycloak:nightly"|"identityProviderImage": ""|' \
  -e 's|"devfileRegistryImage": *"quay.io/eclipse/che-devfile-registry:nightly"|"devfileRegistryImage": ""|' \
  -e 's|"pluginRegistryImage": *"quay.io/eclipse/che-plugin-registry:nightly"|"pluginRegistryImage": ""|' \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "s/^  version: ${lastPackageNightlyVersion}/  version: ${RELEASE}/" \
  -e "/^  version: ${RELEASE}/i\ \ replaces: ${packageName}.v${LAST_RELEASE_VERSION}" \
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

  pushd "${CURRENT_DIR}" || true

  source ${BASE_DIR}/addDigests.sh -w ${BASE_DIR} \
                -t "${RELEASE}" \
                -s "${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"

  popd || true

  if [[ -n "${PRE_RELEASE_CSV}" ]] && [[ -n "${PRE_RELEASE_CRD}" ]]; then
    diff -u "${PRE_RELEASE_CSV}" "${RELEASE_CSV}" > "${RELEASE_CSV}.diff" || true
    diff -u "${PRE_RELEASE_CRD}" "${RELEASE_CRD}" > "${RELEASE_CRD}.diff" || true
  fi
done

echo "[INFO] Release bundles successfully created."
