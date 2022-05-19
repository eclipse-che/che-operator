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

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '--release-version') RELEASE=$2; shift 1;;
    '--channel') CHANNEL=$2; shift 1;;
  esac
  shift 1
done

OPERATOR_DIR=$(dirname $(dirname $(readlink -f "${BASH_SOURCE[0]}")))
BASE_DIR="${OPERATOR_DIR}/olm"

source ${BASE_DIR}/check-yq.sh

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
  mkdir -p "${STABLE_BUNDLE_PATH}/manifests" "${STABLE_BUNDLE_PATH}/generated/openshift" "${STABLE_BUNDLE_PATH}/metadata"
  PRE_RELEASE_CSV="${STABLE_BUNDLE_PATH}/generated/openshift/che-operator.clusterserviceversion.yaml"
  PRE_RELEASE_CHE_CRD="${STABLE_BUNDLE_PATH}/generated/openshift/org_v1_che_crd.yaml"

  wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml" \
      -q -O "${PRE_RELEASE_CSV}"
  wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-openshift/manifests/org_v1_che_crd.yaml" \
      -q -O "${PRE_RELEASE_CHE_CRD}"
}

if [[ -z "$RELEASE" ]] || [[ -z "$CHANNEL" ]]; then
  echo "One of the following required parameters is missing"
  echo "--release-version <RELEASE> --channel <CHANNEL>"
  exit 1
fi


source ${BASE_DIR}/olm.sh
echo "[INFO] Creating release '${RELEASE}'"

NEXT_BUNDLE_PATH=$(getBundlePath "next")

LAST_NEXT_CSV="${NEXT_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
lastPackageNextVersion=$(yq -r ".spec.version" "${LAST_NEXT_CSV}")
echo "[INFO] Last package next version: ${lastPackageNextVersion}"

STABLE_BUNDLE_PATH=$(getBundlePath $CHANNEL)
RELEASE_CSV="${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
RELEASE_CHE_CRD="${STABLE_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"

MANAGER_YAML="${OPERATOR_DIR}/config/manager/manager.yaml"

setLatestReleasedVersion
downloadLatestReleasedBundleCRCRD
packageName=$(getPackageName)

echo "[INFO] Will create release '${RELEASE}' from next version ${lastPackageNextVersion}'"

sed \
-e 's/imagePullPolicy: *Always/imagePullPolicy: IfNotPresent/' \
-e 's/"cheImageTag": *"next"/"cheImageTag": ""/' \
-e 's|quay.io/eclipse/che-dashboard:next|quay.io/eclipse/che-dashboard:'${RELEASE}'|' \
-e 's|"devfileRegistryImage": *"quay.io/eclipse/che-devfile-registry:next"|"devfileRegistryImage": ""|' \
-e 's|"pluginRegistryImage": *"quay.io/eclipse/che-plugin-registry:next"|"pluginRegistryImage": ""|' \
-e "/^  replaces: ${packageName}.v.*/d" \
-e "s/^  version: ${lastPackageNextVersion}/  version: ${RELEASE}/" \
-e "s/: next/: ${RELEASE}/" \
-e "s/:next/:${RELEASE}/" \
-e "s/${lastPackageNextVersion}/${RELEASE}/" \
-e "s/createdAt:.*$/createdAt: \"$(date -u +%FT%TZ)\"/" "${LAST_NEXT_CSV}" > "${RELEASE_CSV}"

cp "${NEXT_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml" "${RELEASE_CHE_CRD}"
cp -rf "${NEXT_BUNDLE_PATH}/bundle.Dockerfile" "${STABLE_BUNDLE_PATH}"
cp -rf "${NEXT_BUNDLE_PATH}/metadata" "${STABLE_BUNDLE_PATH}"
cp -rf "${NEXT_BUNDLE_PATH}/tests" "${STABLE_BUNDLE_PATH}"

ANNOTATION_METADATA_YAML="${STABLE_BUNDLE_PATH}/metadata/annotations.yaml"
sed \
-e 's/operators.operatorframework.io.bundle.channels.v1: .*/operators.operatorframework.io.bundle.channels.v1: '$CHANNEL'/' \
-e 's/operators.operatorframework.io.bundle.channel.default.v1: .*/operators.operatorframework.io.bundle.channel.default.v1: '$CHANNEL'/' \
-i "${ANNOTATION_METADATA_YAML}"

BUNDLE_DOCKERFILE="${STABLE_BUNDLE_PATH}/bundle.Dockerfile"
sed \
-e 's/LABEL operators.operatorframework.io.bundle.channels.v1=.*/LABEL operators.operatorframework.io.bundle.channels.v1='$CHANNEL'/' \
-e 's/LABEL operators.operatorframework.io.bundle.channel.default.v1=.*/LABEL operators.operatorframework.io.bundle.channel.default.v1='$CHANNEL'/' \
-i "${BUNDLE_DOCKERFILE}"

pushd "${CURRENT_DIR}" || exit 1
source ${BASE_DIR}/addDigests.sh -w ${BASE_DIR} \
              -t "${RELEASE}" \
              -s "${RELEASE_CSV}" \
              -o "${MANAGER_YAML}"
popd || exit 1

pushd "${OPERATOR_DIR}" || exit 1
make add-license-download
make add-license "${RELEASE_CSV}"
make add-license "${MANAGER_YAML}"
popd || exit 1

if [[ -n "${PRE_RELEASE_CSV}" ]] && [[ -n "${PRE_RELEASE_CHE_CRD}" ]]; then
  diff -u "${PRE_RELEASE_CSV}" "${RELEASE_CSV}" > "${RELEASE_CSV}.diff" || true
  diff -u "${PRE_RELEASE_CHE_CRD}" "${RELEASE_CHE_CRD}" > "${RELEASE_CHE_CRD}.diff" || true
fi

echo "[INFO] Release bundles successfully created."
