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

# 'next' and 'insiders' are both unreleased rolling editor definition variants
# and are treated the same way by the release flow
EDITOR_DEFINITION_VARIANTS=(next insiders)

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
  echo "  release              Remove unreleased variant ('next', 'insiders') editor definitions that have a 'latest'"
  echo "                       counterpart, tag 'next', 'insiders', 'latest' quay.io/che-incubator images with the"
  echo "                       release version, and promote the remaining variant editor definitions to 'latest'"
  echo "  update-manager-yaml  Update the operator deployment file (manager.yaml) with the RELATED_IMAGE env vars"
  echo
  echo "Options:"
  echo "  --version VERSION     Specify the release version used to tag the images"
  echo "  --yaml-path PATH      Specify the YAML path to the operator deployment file"
  echo "  --help, -h            Show this help message and exit"
  echo
  echo "Examples:"
  echo "  $0 release --version 7.120.0"
  echo "  $0 update-manager-yaml --yaml-path config/manager/manager.yaml"
}

# Remove the unreleased variant definitions with a 'latest' counterpart before tagging: they are not released,
# so their images must neither be tagged nor take part in the digest conflict check
removeRedundantEditorDefinitions() {
  for VARIANT in "${EDITOR_DEFINITION_VARIANTS[@]}"; do
    for FILE in "${EDITORS_DEFINITIONS_DIR}"/*-"${VARIANT}".yaml; do
      [[ -e "${FILE}" ]] || continue

      LATEST_FILE="${FILE%-${VARIANT}.yaml}-latest.yaml"
      if [[ -f "${LATEST_FILE}" ]]; then
        echo "[INFO] removeRedundantEditorDefinitions :: Removing ${FILE}"
        rm -f "${FILE}"
      fi
    done
  done
}

# Keep copies of the original definitions to store the promotion diffs in the 'diffs' directory.
# Must run before tagEditorDefinitionImages so the diffs capture the full change,
# including the image pinning to the release version
snapshotPromotableEditorDefinitions() {
  EDITOR_DEFINITIONS_SNAPSHOT_DIR=$(mktemp -d)
  trap 'rm -rf "${EDITOR_DEFINITIONS_SNAPSHOT_DIR}"' EXIT

  for VARIANT in "${EDITOR_DEFINITION_VARIANTS[@]}"; do
    for FILE in "${EDITORS_DEFINITIONS_DIR}"/*-"${VARIANT}".yaml; do
      [[ -e "${FILE}" ]] || continue
      cp "${FILE}" "${EDITOR_DEFINITIONS_SNAPSHOT_DIR}/$(basename "${FILE}")"
    done
  done
}

promoteEditorDefinitions() {
  for VARIANT in "${EDITOR_DEFINITION_VARIANTS[@]}"; do
    for FILE in "${EDITORS_DEFINITIONS_DIR}"/*-"${VARIANT}".yaml; do
      [[ -e "${FILE}" ]] || continue

      LATEST_FILE="${FILE%-${VARIANT}.yaml}-latest.yaml"
      if [[ -f "${LATEST_FILE}" ]]; then
        echo "[ERROR] Latest editor definitions already exist: ${LATEST_FILE}"
        exit 1
      fi

      echo "[INFO] promoteEditorDefinitions :: Promoting ${FILE} to ${LATEST_FILE}"

      FILE_COPY="${EDITOR_DEFINITIONS_SNAPSHOT_DIR}/$(basename "${FILE}")"

      # 'sed' is preferred over 'yq' to keep the original formatting of the definition
      # and make the promotion diff contain only the meaningful changes.
      # The first letter is matched case-insensitively to handle both 'next' and 'Insiders'
      SUFFIX_RE="[${VARIANT^}${VARIANT,}]${VARIANT:1}"

      # Unwrap the '- Insiders\n    build' style folded scalar continuations first
      sed -zi "s| - ${SUFFIX_RE}\n\([[:space:]]*\)build| - latest|g" "${FILE}"
      sed -i \
        -e "s|^\([[:space:]]*version: \)${VARIANT}\$|\1latest|" \
        -e "s| - ${SUFFIX_RE} build\$| - latest|" \
        -e "s| - ${SUFFIX_RE}\$| - latest|" \
        "${FILE}"
      mv "${FILE}" "${LATEST_FILE}"

      EDITOR_NAME=$(basename "${FILE}" "-${VARIANT}.yaml")

      mkdir -p "${EDITORS_DEFINITIONS_DIFFS_DIR}"
      # 'diff' returns 1 when the files differ, which is the expected outcome here
      diff -u \
        --label "a/editors-definitions/$(basename "${FILE}")" \
        --label "b/editors-definitions/$(basename "${LATEST_FILE}")" \
        "${FILE_COPY}" "${LATEST_FILE}" > "${EDITORS_DEFINITIONS_DIFFS_DIR}/${EDITOR_NAME}.diff" || [[ $? -eq 1 ]]
    done
  done
}

tagEditorDefinitionImages() {
  VARIANTS_PATTERN=$(IFS='|'; echo "${EDITOR_DEFINITION_VARIANTS[*]}")
  EDITOR_IMAGES=$(
    for FILE in "${EDITORS_DEFINITIONS_DIR}"/*.yaml; do
      yq -r '.components[] | select(.container != null) | .container.image' "${FILE}"
    done | sort -u | grep -E "^quay\.io/che-incubator/.+:(${VARIANTS_PATTERN}|latest)$" || true
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
      # Update only the files referencing the image. The image is expected at the end of the line,
      # hence the '$' anchor (not usable with 'grep -F').
      # 'sed' is preferred over 'yq' to keep the original formatting of the definition
      if grep -q "${IMAGE}$" "${FILE}"; then
        sed -i "s|${IMAGE}\$|${VERSIONED_IMAGE}|" "${FILE}"
      fi
    done
  done <<< "${EDITOR_IMAGES}"
}

release() {
  if [[ ! ${VERSION} ]]; then usage; exit 1; fi
  removeRedundantEditorDefinitions
  snapshotPromotableEditorDefinitions
  tagEditorDefinitionImages
  promoteEditorDefinitions
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
