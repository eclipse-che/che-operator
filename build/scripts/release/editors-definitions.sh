#!/bin/bash
#
# Copyright (c) 2019-2024 Red Hat, Inc.
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

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
EDITORS_DEFINITIONS_DIR="${OPERATOR_REPO}/editors-definitions"

init() {
  unset VERSION
  unset YAML_PATH

  COMMAND=$1
  shift

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--version') VERSION=$2; shift 1;;
      '--yaml-path') YAML_PATH=$2; shift 1;;
    esac
    shift 1
  done
}

usage () {
  echo "Usage: $0 COMMAND OPTIONS"
  echo
  echo "Commands:"
  echo "  release              Update the che-code-injector image in the editor definitions"
  echo "  update-manager-yaml  Update the operator deployment file (manager.yaml) with the RELATED_IMAGE env vars"
  echo
  echo "Options:"
  echo "  --version VERSION     Specify the version of the che-code-injector image"
  echo "  --yaml-path PATH      Specify the YAML path to the operator deployment file"
  echo "  --help, -h            Show this help message and exit"
  echo
  echo "Examples:"
  echo "  $0 release --version 0.0.1"
  echo "  $0 update-manager-yaml --yaml-path config/manager/manager.yaml"
}

release() {
  if [[ ! ${VERSION} ]]; then usage; exit 1; fi
  yq -riY "(.components[] | select(.name==\"che-code-injector\") | .container.image) = \"quay.io/che-incubator/che-code:${VERSION}\"" "${EDITORS_DEFINITIONS_DIR}/che-code-latest.yaml"
}

updateOperatorDeploymentEnvVar() {
  if [[ ! ${YAML_PATH} ]]; then usage; exit 1; fi
  yq -riY "(.spec.template.spec.containers[0].env ) += [$(printEnvVars)]" "${YAML_PATH}"
}

printEnvVars() {
  RELATED_IMAGES_ENV=""
  for EDITOR_DEFINITION_FILE in $(find "${EDITORS_DEFINITIONS_DIR}" -name "*.yaml"); do
    NAME=$(yq -r '.metadata.name' "${EDITOR_DEFINITION_FILE}")
    VERSION=$(yq -r '.metadata.attributes.version' "${EDITOR_DEFINITION_FILE}")
    for COMPONENT in $(yq -r '.components[] | .name' "${EDITOR_DEFINITION_FILE}"); do
      ENV_VALUE=$(yq -r ".components[] | select(.name==\"${COMPONENT}\") | .container.image" "${EDITOR_DEFINITION_FILE}")
      ENV_NAME=$(echo "RELATED_IMAGE_editor_definition_${NAME}_${VERSION}_${COMPONENT}" | sed 's|[-\.]|_|g')

      if [[ ! ${ENV_VALUE} == "null" ]]; then
        ENV="{ name: \"${ENV_NAME}\", value: \"${ENV_VALUE}\"}"
        if [[ -z ${RELATED_IMAGES_ENV} ]]; then
            RELATED_IMAGES_ENV="${ENV}"
        else
            RELATED_IMAGES_ENV="${RELATED_IMAGES_ENV}, ${ENV}"
        fi
      fi
    done
  done

  echo "${RELATED_IMAGES_ENV}"
}

init "$@"
pushd "${OPERATOR_REPO}" >/dev/null
case $COMMAND in
      'release') release;;
      'update-manager-yaml'|'add-env-vars') updateOperatorDeploymentEnvVar;;
      *) usage; exit 1;;
esac
popd >/dev/null
