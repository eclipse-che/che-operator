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
  PRE_RELEASE_CHE_CRD="${STABLE_BUNDLE_PATH}/generated/${platform}/org_v1_che_crd.yaml"
  PRE_RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD="${STABLE_BUNDLE_PATH}/generated/${platform}/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
  PRE_RELEASE_CHE_BACKUP_CRD="${STABLE_BUNDLE_PATH}/generated/${platform}/org.eclipse.che_checlusterbackups_crd.yaml"
  PRE_RELEASE_CHE_RESTORE_CRD="${STABLE_BUNDLE_PATH}/generated/${platform}/org.eclipse.che_checlusterrestores_crd.yaml"

  compareResult=$(pysemver compare "${LAST_RELEASE_VERSION}" "7.34.0")
  if [ "${compareResult}" == "1" ]; then
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-${platform}/manifests/che-operator.clusterserviceversion.yaml" \
        -q -O "${PRE_RELEASE_CSV}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-${platform}/manifests/org_v1_che_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_CRD}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-${platform}/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-${platform}/manifests/org.eclipse.che_checlusterbackups_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_BACKUP_CRD}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/bundle/stable/eclipse-che-preview-${platform}/manifests/org.eclipse.che_checlusterrestores_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_RESTORE_CRD}"
  else
    # don't exit immediately if some resources are absent
    set +e
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/che-operator.clusterserviceversion.yaml" \
        -q -O "${PRE_RELEASE_CSV}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/org_v1_che_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_CRD}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/org.eclipse.che_checlusterbackups_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_BACKUP_CRD}"
    wget "https://raw.githubusercontent.com/eclipse-che/che-operator/${LAST_RELEASE_VERSION}/deploy/olm-catalog/stable/eclipse-che-preview-${platform}/manifests/org.eclipse.che_checlusterrestores_crd.yaml" \
        -q -O "${PRE_RELEASE_CHE_RESTORE_CRD}"
    set -e
  fi
}

if [[ -z "$RELEASE" ]] || [[ -z "$CHANNEL" ]]; then
  echo "One of the following required parameters is missing"
  echo "--release-version <RELEASE> --channel <CHANNEL>"
  exit 1
fi


for platform in 'kubernetes' 'openshift'
do
  source ${BASE_DIR}/olm.sh
  echo "[INFO] Creating release '${RELEASE}' for platform '${platform}'"

  if [[ ${CHANNEL} == "tech-preview-stable-all-namespaces" ]] && [[ ${platform} == "kubernetes" ]];then
    continue
  fi

  if [[ ${CHANNEL} == "tech-preview-stable-all-namespaces" ]]; then
    NEXT_BUNDLE_PATH=$(getBundlePath "${platform}" "next-all-namespaces")
  else
    NEXT_BUNDLE_PATH=$(getBundlePath "${platform}" "next")
  fi

  LAST_NEXT_CSV="${NEXT_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  lastPackageNextVersion=$(yq -r ".spec.version" "${LAST_NEXT_CSV}")
  echo "[INFO] Last package next version: ${lastPackageNextVersion}"

  STABLE_BUNDLE_PATH=$(getBundlePath "${platform}" $CHANNEL)
  RELEASE_CSV="${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  RELEASE_CHE_CRD="${STABLE_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
  RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD="${STABLE_BUNDLE_PATH}/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
  RELEASE_CHE_BACKUP_CRD="${STABLE_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterbackups_crd.yaml"
  RELEASE_CHE_RESTORE_CRD="${STABLE_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterrestores_crd.yaml"

  MANAGER_YAML="${OPERATOR_DIR}/config/manager/manager.yaml"

  setLatestReleasedVersion
  downloadLatestReleasedBundleCRCRD
  packageName=$(getPackageName "${platform}")

  echo "[INFO] Will create release '${RELEASE}' from next version ${lastPackageNextVersion}'"

  sed \
  -e 's/imagePullPolicy: *Always/imagePullPolicy: IfNotPresent/' \
  -e 's/"cheImageTag": *"next"/"cheImageTag": ""/' \
  -e 's|quay.io/eclipse/che-dashboard:next|quay.io/eclipse/che-dashboard:'${RELEASE}'|' \
  -e 's|"identityProviderImage": *"quay.io/eclipse/che-keycloak:next"|"identityProviderImage": ""|' \
  -e 's|"devfileRegistryImage": *"quay.io/eclipse/che-devfile-registry:next"|"devfileRegistryImage": ""|' \
  -e 's|"pluginRegistryImage": *"quay.io/eclipse/che-plugin-registry:next"|"pluginRegistryImage": ""|' \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "s/^  version: ${lastPackageNextVersion}/  version: ${RELEASE}/" \
  -e "/^  version: ${RELEASE}/i\ \ replaces: ${packageName}.v${LAST_RELEASE_VERSION}" \
  -e "s/: next/: ${RELEASE}/" \
  -e "s/:next/:${RELEASE}/" \
  -e "s/${lastPackageNextVersion}/${RELEASE}/" \
  -e "s/createdAt:.*$/createdAt: \"$(date -u +%FT%TZ)\"/" "${LAST_NEXT_CSV}" > "${RELEASE_CSV}"

  if [[ ${CHANNEL} == "tech-preview-stable-all-namespaces" ]];then
    # Set tech-preview-stable-all-namespaces versions
    yq -Yi '.spec.replaces |= "'${packageName}'.v'$LAST_RELEASE_VERSION'-all-namespaces"' ${RELEASE_CSV}
    yq -Yi '.spec.version |= "'${RELEASE}'-all-namespaces"' ${RELEASE_CSV}
    yq -Yi '.metadata.name |= "eclipse-che-preview-openshift.v'${RELEASE}'-all-namespaces"' ${RELEASE_CSV}
  fi

  # Remove from devWorkspace in stable channel and hide the value from UI
  if [[ ${CHANNEL} == "stable" ]];then
    CR_SAMPLE=$(yq ".metadata.annotations.\"alm-examples\" | fromjson | del( .[] | select(.kind == \"CheCluster\") | .spec.devWorkspace)" "${RELEASE_CSV}" | sed -r 's/"/\\"/g')
    yq -rY " (.metadata.annotations.\"alm-examples\") = \"${CR_SAMPLE}\"" "${RELEASE_CSV}" > "${RELEASE_CSV}.old"
    yq -Yi '.spec.customresourcedefinitions.owned[] |= (select(.name == "checlusters.org.eclipse.che").specDescriptors += [{"path":"devWorkspace", "x-descriptors": ["urn:alm:descriptor:com.tectonic.ui:hidden"]}])' "${RELEASE_CSV}.old"
    mv "${RELEASE_CSV}.old" "${RELEASE_CSV}"

    if [[ ${platform} == "openshift" ]];then
      yq -rYi "(.spec.install.spec.deployments [] | select(.name == \"che-operator\") | .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"ALLOW_DEVWORKSPACE_ENGINE\") | .value ) = \"false\"" ${RELEASE_CSV}
    fi
  fi

  cp "${NEXT_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml" "${RELEASE_CHE_CRD}"
  cp "${NEXT_BUNDLE_PATH}/manifests/org.eclipse.che_chebackupserverconfigurations.yaml" "${RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}"
  cp "${NEXT_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterbackups.yaml" "${RELEASE_CHE_BACKUP_CRD}"
  cp "${NEXT_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterrestores.yaml" "${RELEASE_CHE_RESTORE_CRD}"
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
  make add-license "${RELEASE_CSV}"
  make add-license "${MANAGER_YAML}"
  popd || exit 1

  if [[ -n "${PRE_RELEASE_CSV}" ]] && [[ -n "${PRE_RELEASE_CHE_CRD}" ]]; then
    diff -u "${PRE_RELEASE_CSV}" "${RELEASE_CSV}" > "${RELEASE_CSV}.diff" || true
    diff -u "${PRE_RELEASE_CHE_CRD}" "${RELEASE_CHE_CRD}" > "${RELEASE_CHE_CRD}.diff" || true
  fi
  if [[ -n "${PRE_RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}" ]]; then
    diff -u "${PRE_RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}" "${RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}" > "${RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}.diff" || true
  fi
  if [[ -n "${PRE_RELEASE_CHE_BACKUP_CRD}" ]]; then
    diff -u "${PRE_RELEASE_CHE_BACKUP_CRD}" "${RELEASE_CHE_BACKUP_CRD}" > "${RELEASE_CHE_BACKUP_CRD}.diff" || true
  fi
  if [[ -n "${PRE_RELEASE_CHE_RESTORE_CRD}" ]]; then
    diff -u "${PRE_RELEASE_CHE_RESTORE_CRD}" "${RELEASE_CHE_RESTORE_CRD}" > "${RELEASE_CHE_RESTORE_CRD}.diff" || true
  fi
done

echo "[INFO] Release bundles successfully created."
