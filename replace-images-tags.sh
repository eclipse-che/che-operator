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
#
# Updates images into:
# - deploy/operator.yaml
# - deploy/operator-local.yaml
# Usage:
#   ./release-operator-code.sh <RELEASE> <CHE_RELEASE_BRANCH>

set -e
set -x

function init() {
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
  RELEASE="$1"
  CHE_RELEASE_BRANCH="$2"
}

function getLatestUbi8MinimalImage() {
  ubiMinimal8Version=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.version')
  ubiMinimal8Release=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.release')
  UBI8_MINIMAL_IMAGE="registry.access.redhat.com/ubi8-minimal:"$ubiMinimal8Version"-"$ubiMinimal8Release
  skopeo inspect docker://$UBI8_MINIMAL_IMAGE > /dev/null
}

function replaceImageTag() {
    echo "${1}" | sed -e "s/\(.*:\).*/\1${2}/"
}

replaceImagesTags() {
  OPERATOR_YAML="${BASE_DIR}"/deploy/operator.yaml
  OPERATOR_LOCAL_YAML="${BASE_DIR}"/deploy/operator-local.yaml

  lastDefaultCheServerImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value" "${OPERATOR_YAML}")
  lastDefaultKeycloakImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value" "${OPERATOR_YAML}")
  lastDefaultPluginRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value" "${OPERATOR_YAML}")
  lastDefaultDevfileRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value" "${OPERATOR_YAML}")

  wget https://raw.githubusercontent.com/eclipse/che/${CHE_RELEASE_BRANCH}/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties
  PLUGIN_BROKER_METADATA_IMAGE_RELEASE=$(cat /tmp/che.properties| grep "che.workspace.plugin_broker.metadata.image" | cut -d = -f2)
  PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE=$(cat /tmp/che.properties | grep "che.workspace.plugin_broker.artifacts.image" | cut -d = -f2)
  JWT_PROXY_IMAGE_RELEASE=$(cat /tmp/che.properties | grep "che.server.secure_exposer.jwtproxy.image" | cut -d = -f2)
  CHE_SERVER_IMAGE_REALEASE=$(replaceImageTag "${lastDefaultCheServerImage}" "${RELEASE}")
  KEYCLOAK_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultKeycloakImage}" "${RELEASE}")
  PLUGIN_REGISTRY_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultPluginRegistryImage}" "${RELEASE}")
  DEVFILE_REGISTRY_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultDevfileRegistryImage}" "${RELEASE}")
  rm /tmp/che.properties

  NEW_OPERATOR_YAML="${OPERATOR_YAML}.new"
  NEW_OPERATOR_LOCAL_YAML="${OPERATOR_LOCAL_YAML}.new"
  # copy licence header
  eval head -10 "${OPERATOR_YAML}" > ${NEW_OPERATOR_YAML}
  eval head -10 "${OPERATOR_LOCAL_YAML}" > ${NEW_OPERATOR_LOCAL_YAML}

  cat "${OPERATOR_YAML}" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\") | .image ) = \"quay.io/eclipse/che-operator:${RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"CHE_VERSION\") | .value ) = \"${RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value ) = \"${CHE_SERVER_IMAGE_REALEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value ) = \"${KEYCLOAK_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value ) = \"${PLUGIN_REGISTRY_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value ) = \"${DEVFILE_REGISTRY_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_pvc_jobs\") | .value ) = \"${UBI8_MINIMAL_IMAGE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_metadata\") | .value ) = \"${PLUGIN_BROKER_METADATA_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_artifacts\") | .value ) = \"${PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"${JWT_PROXY_IMAGE_RELEASE}\"" \
  >> "${NEW_OPERATOR_YAML}"
  mv "${NEW_OPERATOR_YAML}" "${OPERATOR_YAML}"

  cat "${OPERATOR_LOCAL_YAML}" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\") | .image ) = \"quay.io/eclipse/che-operator:${RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"CHE_VERSION\") | .value ) = \"${RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value ) = \"${CHE_SERVER_IMAGE_REALEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value ) = \"${KEYCLOAK_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value ) = \"${PLUGIN_REGISTRY_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value ) = \"${DEVFILE_REGISTRY_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_pvc_jobs\") | .value ) = \"${UBI8_MINIMAL_IMAGE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_metadata\") | .value ) = \"${PLUGIN_BROKER_METADATA_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_artifacts\") | .value ) = \"${PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"${JWT_PROXY_IMAGE_RELEASE}\"" \
  >> "${NEW_OPERATOR_LOCAL_YAML}"
  mv "${NEW_OPERATOR_LOCAL_YAML}" "${OPERATOR_LOCAL_YAML}"

  DOCKERFILE=${BASE_DIR}/Dockerfile
  sed -i 's|registry.access.redhat.com/ubi8-minimal:.*|'${UBI8_MINIMAL_IMAGE}'|g' $DOCKERFILE
}

init "$@"
getLatestUbi8MinimalImage "$@"
replaceImagesTags "$@"
