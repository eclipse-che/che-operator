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

usage () {
  echo "Usage: $0 COMMAND OPTIONS"
  echo
  echo "Commands:"
  echo "  update-csv-yaml      Update the operator deployment in the CSV file with the RELATED_IMAGE env vars"
  echo "  update-manager-yaml  Update the operator deployment file (manager.yaml) with the RELATED_IMAGE env vars"
  echo
  echo "Options:"
  echo "  --index-json-url URL  Specify the index JSON URL with samples"
  echo "  --yaml-path PATH      Specify the YAML path to CSV or operator deployment file"
  echo "  --help, -h            Show this help message and exit"
  echo
  echo "Examples:"
  echo "  $0 update-csv-yaml --index-json-url https://raw.githubusercontent.com/eclipse-che/che-dashboard/main/packages/devfile-registry/air-gap/index.json --yaml-path config/manager/manager.yaml"
  echo "  $0 update-manager-yaml --index-json-url https://raw.githubusercontent.com/eclipse-che/che-dashboard/main/packages/devfile-registry/air-gap/index.json --yaml-path bundle/next/eclipse-che/manifests/che-operator.clusterserviceversion.yaml"
}

init() {
  unset SAMPLES_INDEX_JSON_URL
  unset YAML_PATH

  COMMAND=$1
  shift

  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--index-json-url') SAMPLES_INDEX_JSON_URL=$2; shift 1;;
      '--yaml-path') YAML_PATH=$2; shift 1;;
      '--help'|'-h') usage; exit 0;;
    esac
    shift 1
  done

  if [[ -z "${SAMPLES_INDEX_JSON_URL}" ]] || [[ -z "${YAML_PATH}" ]]; then
    usage
    exit 1
  fi
}

updateOperatorDeploymentEnvVar() {
  yq -riY "(.spec.template.spec.containers[0].env ) += [$(printEnvVars)]" "${YAML_PATH}"
}

updateCSVOperatorDeploymentEnvVarYaml() {
  yq -riY "(.spec.install.spec.deployments[0].spec.template.spec.containers[0].env ) += [$(printEnvVars)]" "${YAML_PATH}"
}

printEnvVars() {
  # Get the list of samples urls
  curl -sSL "${SAMPLES_INDEX_JSON_URL}" --output /tmp/samples.json
  if [[ $(cat /tmp/samples.json) == *"404"* ]] || [[ $(cat /tmp/samples.json) == *"Not Found"* ]]; then
      echo "[ERROR] Could not load ${SAMPLES_INDEX_JSON_URL}"
      exit 1
  fi
  SAMPLE_URLS=($(yq -r '.[] | .url' /tmp/samples.json))

  RELATED_IMAGES_ENV=""
  for SAMPLE_URL in "${SAMPLE_URLS[@]}"; do
    # Fetch the corresponding devfile.yaml
    SAMPLE_ORG="$(echo "${SAMPLE_URL}" | cut -d '/' -f 4)"
    SAMPLE_REPOSITORY="$(echo "${SAMPLE_URL}" | cut -d '/' -f 5)"
    SAMPLE_REF="$(echo "${SAMPLE_URL}" | cut -d '/' -f 7)"
    DEVFILE_SAMPLE_URL="https://raw.githubusercontent.com/${SAMPLE_ORG}/${SAMPLE_REPOSITORY}/${SAMPLE_REF}/devfile.yaml"
    curl -sSL "${DEVFILE_SAMPLE_URL}" --output /tmp/devfile.yaml
    if [[ $(cat /tmp/devfile.yaml) == *"404"* ]] || [[ $(cat /tmp/devfile.yaml) == *"Not Found"* ]]; then
        echo "[ERROR] Could not load ${DEVFILE_SAMPLE_URL}"
        exit 1
    fi

    # Iterate over the components and add the RELATED_IMAGE env vars
    CONTAINER_INDEX=0
    while [ "${CONTAINER_INDEX}" -lt "$(yq -r '.components | length' "/tmp/devfile.yaml")" ]; do
      CONTAINER_IMAGE_ENV_NAME=""
      CONTAINER_IMAGE=$(yq -r '.components['${CONTAINER_INDEX}'].container.image' /tmp/devfile.yaml)
      if [[ ${CONTAINER_IMAGE} == *"@"*  ]]; then
        # We don't need to encode the image name if it already contains a digest
        # So, make a simple env var name containing sample and component names
        SAMPLE_NAME=$(yq -r '.metadata.name' /tmp/devfile.yaml | sed 's|-|_|g')
        COMPONENT_NAME=$(yq -r '.components['${CONTAINER_INDEX}'].name' /tmp/devfile.yaml | sed 's|-|_|g')
        CONTAINER_IMAGE_ENV_NAME="RELATED_IMAGE_sample_${SAMPLE_NAME}_${COMPONENT_NAME}"
      elif [[ ${CONTAINER_IMAGE} == *":"* ]]; then
        # Encode the image name if it contains a tag
        # It is used in dashboard to replace the image in the devfile.yaml at startup
        CONTAINER_IMAGE_ENV_NAME="RELATED_IMAGE_sample_encoded_$(echo "${CONTAINER_IMAGE}" | base64 -w 0 | sed 's|=|____|g')"
      fi

      if [[ -n ${CONTAINER_IMAGE_ENV_NAME} ]]; then
        ENV="{name: \"${CONTAINER_IMAGE_ENV_NAME}\", value: \"${CONTAINER_IMAGE}\"}"
        if [[ -z ${RELATED_IMAGES_ENV} ]]; then
          RELATED_IMAGES_ENV="${ENV}"
        elif [[ ! ${RELATED_IMAGES_ENV} =~ ${CONTAINER_IMAGE_ENV_NAME} ]]; then
          RELATED_IMAGES_ENV="${RELATED_IMAGES_ENV}, ${ENV}"
        fi
      fi

      CONTAINER_INDEX=$((CONTAINER_INDEX+1))
    done
  done

  echo "${RELATED_IMAGES_ENV}"
}

init "$@"
pushd "${OPERATOR_REPO}" >/dev/null
case $COMMAND in
      'update-csv-yaml') updateCSVOperatorDeploymentEnvVarYaml;;
      'update-manager-yaml') updateOperatorDeploymentEnvVar;;
      *) usage; exit 1;;
esac
popd >/dev/null
