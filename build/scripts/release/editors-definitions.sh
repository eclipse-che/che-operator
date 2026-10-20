#!/bin/bash
#
# Copyright (c) 2019-2026 Red Hat, Inc.
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
EDITORS_DEFINITIONS_DIFFS_DIR="${EDITORS_DEFINITIONS_DIR}/diffs"

command -v skopeo >/dev/null 2>&1 || { echo "[ERROR] skopeo is not installed. Aborting."; exit 1; }

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
  echo "  release              Remove 'next' editor definitions that have a 'latest' counterpart, tag 'next',"
  echo "                       'insiders' and 'latest' quay.io/che-incubator images with the release version,"
  echo "                       and promote the remaining 'next' editor definitions to 'latest'"
  echo "  update-manager-yaml  Update the operator deployment file (manager.yaml) with the RELATED_IMAGE env vars"
  echo
  echo "Options:"
  echo "  --version VERSION     Specify the release version used to tag the images"
  echo "  --yaml-path PATH      Specify the YAML path to the operator deployment file"
  echo "  --help, -h            Show this help message and exit"
  echo
  echo "Examples:"
  echo "  $0 release --version 0.0.1"
  echo "  $0 update-manager-yaml --yaml-path config/manager/manager.yaml"
}

# Remove the 'next' definitions with a 'latest' counterpart before tagging: they are not released,
# so their images must neither be tagged nor take part in the digest conflict check
removeRedundantNextEditorDefinitions() {
  for NEXT_FILE in "${EDITORS_DEFINITIONS_DIR}"/*-next.yaml; do
    [[ -e "${NEXT_FILE}" ]] || continue

    LATEST_FILE="${NEXT_FILE%-next.yaml}-latest.yaml"
    INSIDERS_FILE="${NEXT_FILE%-next.yaml}-insiders.yaml"

    if [[ -f "${LATEST_FILE}" && -f "${INSIDERS_FILE}" ]]; then
      echo "[INFO] removeRedundantNextEditorDefinitions :: Removing ${NEXT_FILE}"
      rm -f "${NEXT_FILE}"
    fi
  done
}

# Keep copies of the original definitions to store the promotion diffs in the 'diffs' directory.
# Must run before tagEditorDefinitionImages so the diffs capture the full change,
# including the image pinning to the release version
snapshotPromotableEditorDefinitions() {
  EDITOR_DEFINITIONS_SNAPSHOT_DIR=$(mktemp -d)
  trap 'rm -rf "${EDITOR_DEFINITIONS_SNAPSHOT_DIR}"' EXIT

  for NEXT_FILE in "${EDITORS_DEFINITIONS_DIR}"/*-next.yaml; do
    [[ -e "${NEXT_FILE}" ]] || continue
    cp "${NEXT_FILE}" "${EDITOR_DEFINITIONS_SNAPSHOT_DIR}/$(basename "${NEXT_FILE}")"
  done
}

promoteNextEditorDefinitions() {
  for NEXT_FILE in "${EDITORS_DEFINITIONS_DIR}"/*-next.yaml; do
    [[ -e "${NEXT_FILE}" ]] || continue

    LATEST_FILE="${NEXT_FILE%-next.yaml}-latest.yaml"
    if [[ -f "${LATEST_FILE}" ]]; then
      echo "[ERROR] Latest editor definitions already exist: ${LATEST_FILE}"
      exit 1
    fi

    echo "[INFO] promoteNextEditorDefinitions :: Promoting ${NEXT_FILE} to ${LATEST_FILE}"

    EDITOR_NAME=$(basename "${NEXT_FILE}" -next.yaml)
    NEXT_FILE_COPY="${EDITOR_DEFINITIONS_SNAPSHOT_DIR}/${EDITOR_NAME}-next.yaml"

    yq -riY '(.metadata.attributes.version) = "latest" |
      (.metadata.description) |= sub(" - next$"; " - latest") |
      (.metadata.attributes.title) |= sub(" - next$"; " - latest")' "${NEXT_FILE}"
    mv "${NEXT_FILE}" "${LATEST_FILE}"

    mkdir -p "${EDITORS_DEFINITIONS_DIFFS_DIR}"
    # 'diff' returns 1 when the files differ, which is the expected outcome here
    diff -u \
      --label "a/editors-definitions/${EDITOR_NAME}-next.yaml" \
      --label "b/editors-definitions/${EDITOR_NAME}-latest.yaml" \
      "${NEXT_FILE_COPY}" "${LATEST_FILE}" > "${EDITORS_DEFINITIONS_DIFFS_DIR}/${EDITOR_NAME}.diff" || [[ $? -eq 1 ]]
  done
}

# Tag all 'quay.io/che-incubator' images referenced by the editor definitions with a 'next',
# 'insiders' or 'latest' tag with the release version, and update the definitions to use the
# versioned images.
tagEditorDefinitionImages() {
  EDITOR_IMAGES=$(
    for FILE in "${EDITORS_DEFINITIONS_DIR}"/*.yaml; do
      yq -r '.components[] | select(.container != null) | .container.image' "${FILE}"
    done | sort -u | grep -E '^quay\.io/che-incubator/.+:(next|insiders|latest)$' || true
  )

  if [[ -z "${EDITOR_IMAGES}" ]]; then
    return
  fi

  while read -r IMAGE; do
    VERSIONED_IMAGE="${IMAGE%:*}:${VERSION}"

    if skopeo inspect "docker://${VERSIONED_IMAGE}" >/dev/null 2>&1; then
      echo "[INFO] tagEditorDefinitionImages :: Image ${VERSIONED_IMAGE} already exists. Using the existing tag."
    else
      if ! skopeo inspect "docker://${IMAGE}" >/dev/null 2>&1; then
        echo "[ERROR] tagEditorDefinitionImages :: Source image ${IMAGE} does not exist in the registry. Aborting."
        exit 1
      fi

      if [[ "${DRY_RUN:-}" == "true" ]]; then
        echo "[INFO] tagEditorDefinitionImages :: Dry run is enabled. Skipping tagging of ${IMAGE} as ${VERSIONED_IMAGE}."
      else
        if [[ -z "${QUAY_ECLIPSE_CHE_USERNAME}" || -z "${QUAY_ECLIPSE_CHE_PASSWORD}" ]]; then
          echo "[ERROR] tagEditorDefinitionImages :: QUAY_ECLIPSE_CHE_USERNAME and QUAY_ECLIPSE_CHE_PASSWORD must be set to push ${VERSIONED_IMAGE}. Aborting."
          exit 1
        fi

        echo "[INFO] tagEditorDefinitionImages :: Tagging ${IMAGE} as ${VERSIONED_IMAGE}"
        skopeo copy --all --preserve-digests  \
          --dest-creds "${QUAY_ECLIPSE_CHE_USERNAME}:${QUAY_ECLIPSE_CHE_PASSWORD}" \
          "docker://${IMAGE}" "docker://${VERSIONED_IMAGE}"
      fi
    fi

    echo "[INFO] tagEditorDefinitionImages :: Update editor definitions to use ${VERSIONED_IMAGE}"
    for FILE in "${EDITORS_DEFINITIONS_DIR}"/*.yaml; do
      # Update only the files referencing the image to avoid re-serializing the others
      if grep -q "${IMAGE}\$" "${FILE}"; then
        yq -riY "(.components[] | select(.container != null) | .container.image | select(. == \"${IMAGE}\")) = \"${VERSIONED_IMAGE}\"" "${FILE}"
      fi
    done
  done <<< "${EDITOR_IMAGES}"
}

release() {
  if [[ ! ${VERSION} ]]; then usage; exit 1; fi
  removeRedundantNextEditorDefinitions
  snapshotPromotableEditorDefinitions
  tagEditorDefinitionImages
  promoteNextEditorDefinitions
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
