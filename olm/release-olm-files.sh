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

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '--release-version') RELEASE=$2; shift 1;;
    '--channel') CHANNEL=$2; shift 1;;
    '--dev-workspace-controller-version') DEV_WORKSPACE_CONTROLLER_VERSION=$2; shift 1;;
    '--dev-workspace-che-operator-version') DEV_WORKSPACE_CHE_OPERATOR_VERSION=$2; shift 1;;
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

if [[ -z "$RELEASE" ]] || [[ -z "$RELEASE" ]] || [[ -z "$RELEASE" ]]; then
  echo "One of the following required parameters is missing"
  echo "--release-version $RELEASE"
  echo "--dev-workspace-controller-version $DEV_WORKSPACE_CONTROLLER_VERSION"
  echo "--dev-workspace-che-operator-version $DEV_WORKSPACE_CHE_OPERATOR_VERSION"
  exit 1
fi


for platform in 'kubernetes' 'openshift'
do
  source ${BASE_DIR}/olm.sh
  echo "[INFO] Creating release '${RELEASE}' for platform '${platform}'"

  if [[ ${CHANNEL} == "stable-all-namespaces" ]] && [[ ${platform} == "kubernetes" ]];then
    continue
  fi

  NIGHTLY_BUNDLE_PATH=$(getBundlePath "${platform}" "nightly")
  LAST_NIGHTLY_CSV="${NIGHTLY_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  lastPackageNightlyVersion=$(yq -r ".spec.version" "${LAST_NIGHTLY_CSV}")
  echo "[INFO] Last package nightly version: ${lastPackageNightlyVersion}"

  STABLE_BUNDLE_PATH=$(getBundlePath "${platform}" $CHANNEL)
  RELEASE_CSV="${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  RELEASE_CHE_CRD="${STABLE_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
  RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD="${STABLE_BUNDLE_PATH}/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
  RELEASE_CHE_BACKUP_CRD="${STABLE_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterbackups_crd.yaml"
  RELEASE_CHE_RESTORE_CRD="${STABLE_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterrestores_crd.yaml"

  setLatestReleasedVersion
  downloadLatestReleasedBundleCRCRD
  packageName=$(getPackageName "${platform}")

  echo "[INFO] Will create release '${RELEASE}' from nightly version ${lastPackageNightlyVersion}'"

  sed \
  -e 's/imagePullPolicy: *Always/imagePullPolicy: IfNotPresent/' \
  -e 's/"cheImageTag": *"next"/"cheImageTag": ""/' \
  -e 's|quay.io/eclipse/che-dashboard:next|quay.io/eclipse/che-dashboard:'${RELEASE}'|' \
  -e 's|quay.io/che-incubator/devworkspace-che-operator:next|quay.io/che-incubator/devworkspace-che-operator:'${DEV_WORKSPACE_CHE_OPERATOR_VERSION}'|' \
  -e 's|quay.io/devfile/devworkspace-controller:next|quay.io/devfile/devworkspace-controller:'${DEV_WORKSPACE_CONTROLLER_VERSION}'|' \
  -e 's|"identityProviderImage": *"quay.io/eclipse/che-keycloak:next"|"identityProviderImage": ""|' \
  -e 's|"devfileRegistryImage": *"quay.io/eclipse/che-devfile-registry:next"|"devfileRegistryImage": ""|' \
  -e 's|"pluginRegistryImage": *"quay.io/eclipse/che-plugin-registry:next"|"pluginRegistryImage": ""|' \
  -e "/^  replaces: ${packageName}.v.*/d" \
  -e "s/^  version: ${lastPackageNightlyVersion}/  version: ${RELEASE}/" \
  -e "/^  version: ${RELEASE}/i\ \ replaces: ${packageName}.v${LAST_RELEASE_VERSION}" \
  -e "s/: next/: ${RELEASE}/" \
  -e "s/:next/:${RELEASE}/" \
  -e "s/${lastPackageNightlyVersion}/${RELEASE}/" \
  -e "s/createdAt:.*$/createdAt: \"$(date -u +%FT%TZ)\"/" "${LAST_NIGHTLY_CSV}" > "${RELEASE_CSV}"

  if [[ ${CHANNEL} == "stable-all-namespaces" ]];then
    # Set by default devworkspace enabled
    CR_SAMPLE=$(yq -r ".metadata.annotations.\"alm-examples\"" "${RELEASE_CSV}" | yq -r ".[0] | .spec.devWorkspace.enable |= true | [.]" | sed -r 's/"/\\"/g')
    yq -rY " (.metadata.annotations.\"alm-examples\") = \"${CR_SAMPLE}\"" "${RELEASE_CSV}" > "${RELEASE_CSV}.old"
    mv "${RELEASE_CSV}.old" "${RELEASE_CSV}"

    # Move the suggested namespace to openshift-operators.
    sed -ri 's|operatorframework.io/suggested-namespace: eclipse-che|operatorframework.io/suggested-namespace: openshift-operators|' "${RELEASE_CSV}"

    # Set stable-all-namespaces versions
    yq -Yi '.spec.replaces |= "'${packageName}'.v'$LAST_RELEASE_VERSION'-all-namespaces"' ${RELEASE_CSV}
    yq -Yi '.spec.version |= "'${RELEASE}'-all-namespaces"' ${RELEASE_CSV}
    yq -Yi '.metadata.name |= "eclipse-che-preview-openshift.v'${RELEASE}'-all-namespaces"' ${RELEASE_CSV}

    # Change the install Mode to AllNamespaces by default
    yq -Yi '.spec.installModes[] |= if .type=="OwnNamespace" then .supported |= false else . end' ${RELEASE_CSV}
    yq -Yi '.spec.installModes[] |= if .type=="SingleNamespace" then .supported |= false else . end' ${RELEASE_CSV}
    yq -Yi '.spec.installModes[] |= if .type=="MultiNamespace" then .supported |= false else . end' ${RELEASE_CSV}
    yq -Yi '.spec.installModes[] |= if .type=="AllNamespaces" then .supported |= true else . end' ${RELEASE_CSV}
  fi

  cp "${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml" "${RELEASE_CHE_CRD}"
  cp "${NIGHTLY_BUNDLE_PATH}/manifests/org.eclipse.che_chebackupserverconfigurations.yaml" "${RELEASE_CHE_BACKUP_SERVER_CONFIGURATION_CRD}"
  cp "${NIGHTLY_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterbackups.yaml" "${RELEASE_CHE_BACKUP_CRD}"
  cp "${NIGHTLY_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterrestores.yaml" "${RELEASE_CHE_RESTORE_CRD}"
  cp -rf "${NIGHTLY_BUNDLE_PATH}/bundle.Dockerfile" "${STABLE_BUNDLE_PATH}"
  cp -rf "${NIGHTLY_BUNDLE_PATH}/metadata" "${STABLE_BUNDLE_PATH}"
  cp -rf "${NIGHTLY_BUNDLE_PATH}/tests" "${STABLE_BUNDLE_PATH}"

  ANNOTATION_METADATA_YAML="${STABLE_BUNDLE_PATH}/metadata/annotations.yaml"
  sed \
  -e 's/operators.operatorframework.io.bundle.channels.v1: *nightly/operators.operatorframework.io.bundle.channels.v1: '$CHANNEL'/' \
  -e 's/operators.operatorframework.io.bundle.channel.default.v1: *nightly/operators.operatorframework.io.bundle.channel.default.v1: '$CHANNEL'/' \
  -i "${ANNOTATION_METADATA_YAML}"

  BUNDLE_DOCKERFILE="${STABLE_BUNDLE_PATH}/bundle.Dockerfile"
  sed \
  -e 's/LABEL operators.operatorframework.io.bundle.channels.v1=nightly/LABEL operators.operatorframework.io.bundle.channels.v1='$CHANNEL'/' \
  -e 's/LABEL operators.operatorframework.io.bundle.channel.default.v1=nightly/LABEL operators.operatorframework.io.bundle.channel.default.v1='$CHANNEL'/' \
  -i "${BUNDLE_DOCKERFILE}"

  pushd "${CURRENT_DIR}" || true
  source ${BASE_DIR}/addDigests.sh -w ${BASE_DIR} \
                -t "${RELEASE}" \
                -s "${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml" \
                -o "${OPERATOR_DIR}/config/manager/manager.yaml"
  popd || true

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
