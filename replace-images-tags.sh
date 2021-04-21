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
# Usage:
#   ./release-operator-code.sh <RELEASE_TAG> <CHE_RELEASE_BRANCH>

set -e

function init() {
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
  RELEASE_TAG="$1"
  CHE_RELEASE_BRANCH="$2"
}

function replaceImageTag() {
    echo "${1}" | sed -e "s/\(.*:\).*/\1${2}/"
}

replaceImagesTags() {
  OPERATOR_YAML="${BASE_DIR}"/deploy/operator.yaml

  lastDefaultCheServerImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value" "${OPERATOR_YAML}")
  lastDefaultDashboardImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_dashboard\") | .value" "${OPERATOR_YAML}")
  lastDefaultKeycloakImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value" "${OPERATOR_YAML}")
  lastDefaultPluginRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value" "${OPERATOR_YAML}")
  lastDefaultDevfileRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value" "${OPERATOR_YAML}")

  CHE_SERVER_IMAGE_REALEASE=$(replaceImageTag "${lastDefaultCheServerImage}" "${RELEASE_TAG}")
  DASHBOARD_IMAGE_REALEASE=$(replaceImageTag "${lastDefaultDashboardImage}" "${RELEASE_TAG}")
  KEYCLOAK_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultKeycloakImage}" "${RELEASE_TAG}")
  PLUGIN_REGISTRY_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultPluginRegistryImage}" "${RELEASE_TAG}")
  DEVFILE_REGISTRY_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultDevfileRegistryImage}" "${RELEASE_TAG}")

  NEW_OPERATOR_YAML="${OPERATOR_YAML}.new"
  # copy licence header
  eval head -10 "${OPERATOR_YAML}" > ${NEW_OPERATOR_YAML}

  cat "${OPERATOR_YAML}" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\") | .image ) = \"quay.io/eclipse/che-operator:${RELEASE_TAG}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"CHE_VERSION\") | .value ) = \"${RELEASE_TAG}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server\") | .value ) = \"${CHE_SERVER_IMAGE_REALEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_dashboard\") | .value ) = \"${DASHBOARD_IMAGE_REALEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_keycloak\") | .value ) = \"${KEYCLOAK_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_plugin_registry\") | .value ) = \"${PLUGIN_REGISTRY_IMAGE_RELEASE}\"" | \
  yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_devfile_registry\") | .value ) = \"${DEVFILE_REGISTRY_IMAGE_RELEASE}\"" \
  >> "${NEW_OPERATOR_YAML}"
  mv "${NEW_OPERATOR_YAML}" "${OPERATOR_YAML}"
}

init "$@"
replaceImagesTags "$@"
