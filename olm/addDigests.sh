#!/bin/bash
#
# Copyright (c) 2019-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set +x
set -e

SCRIPTS_DIR=$(cd "$(dirname "$0")"; pwd)
BASE_DIR="$(pwd)"
QUIET=""

PODMAN=$(command -v podman || true)
if [[ ! -x $PODMAN ]]; then
  echo "[WARNING] podman is not installed."
  PODMAN=$(command -v docker)
  if [[ ! -x $PODMAN ]]; then
    echo "[ERROR] docker is not installed. Aborting."; exit 1
  fi
fi
command -v yq >/dev/null 2>&1 || { echo "yq is not installed. Aborting."; exit 1; }

usage () {
	echo "Usage:   $0 [-w WORKDIR] [-s CSV_FILE_PATH] -t [IMAGE_TAG] "
	echo "Example: ./olm/addDigests.sh -w . -s 'deploy/olm-catalog/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml' -t 7.21.1"
}

if [[ $# -lt 1 ]]; then usage; exit; fi

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-w') BASE_DIR="$2"; shift 1;;
    '-s') CSV_FILE="$2"; shift 1;;
    '-t') IMAGE_TAG="$2"; shift 1;;
    '-q') QUIET="-q"; shift 0;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! ${CSV_FILE} ]] || [[ ! $IMAGE_TAG ]]; then usage; exit 1; fi

echo "CSV file: ${CSV_FILE}"
RELATED_IMAGE_PREFIX="RELATED_IMAGE_"

rm -Rf "${BASE_DIR}/generated/csv"
mkdir -p "${BASE_DIR}/generated/csv"

if [ -f "${CSV_FILE}" ];then
  cp -pR "${CSV_FILE}" "${BASE_DIR}/generated/csv"
else
  echo "[ERROR] ${CSV_FILE} was not found"
  exit 1
fi

# shellcheck source=buildDigestMap.sh
source "${SCRIPTS_DIR}/buildDigestMap.sh" -w "${BASE_DIR}" -t "${IMAGE_TAG}" -c "${CSV_FILE}" ${QUIET}

if [[ ! "${QUIET}" ]]; then cat "${BASE_DIR}"/generated/digests-mapping.txt; fi

CSV_FILE_COPY=${BASE_DIR}/generated/csv/$(basename ${CSV_FILE})

echo "[INFO] Generate digest update for CSV file ${CSV_FILE}"
RELATED_IMAGES=""
RELATED_IMAGES_ENV=""
for mapping in $(cat "${BASE_DIR}/generated/digests-mapping.txt")
do
  source=$(echo "${mapping}" | sed -e 's;\(.*\)=.*=.*;\1;')
  # Image with digest.
  dest=$(echo "${mapping}" | sed -e 's;.*=.*=\(.*\);\1;')
  # Image label to set image target. For example: 'devfile-registry-image'
  imageLabel=$(echo "${mapping}" | sed -e 's;.*=\(.*\)=.*;\1;')
  name=$(echo "${dest}" | sed -e 's;.*/\([^\/][^\/]*\)@.*;\1;')
  tagOrDigest=""
  if [[ ${source} == *"@"* ]]; then
    tagOrDigest="@${source#*@}"
  elif [[ ${source} == *":"* ]]; then
    tagOrDigest="${source#*:}"
  fi

  if [[ ${imageLabel} == "plugin-registry-image" ]] || [[ ${imageLabel} == "devfile-registry-image" ]]; then
    # Image tag could contains invalid for Env variable name characters, so let's encode it using base32.
    # But alphabet of base32 uses one invalid for env variable name character '=' at the end of the line, so let's replace it by '_'.
    # To recovery original tag should be done opposite actions: replace '_' to '=', and decode string using 'base32 -d'.
    encodedTag=$(echo "${tagOrDigest}" | base32 -w 0 | tr "=" "_")
    relatedImageEnvName=$(echo "${RELATED_IMAGE_PREFIX}${name}_${imageLabel}_${encodedTag}" | sed -r 's/[-.]/_/g')
    ENV="{ name: \"${relatedImageEnvName}\", value: \"${dest}\"}"
    if [[ -z ${RELATED_IMAGES_ENV} ]]; then
      RELATED_IMAGES_ENV="${ENV}"
    else
      RELATED_IMAGES_ENV="${RELATED_IMAGES_ENV}, ${ENV}"
    fi
  fi

  RELATED_IMAGE="{ name: \"${name}-${tagOrDigest}\", image: \"${dest}\", tag: \"${source}\"}"
  if [[ -z ${RELATED_IMAGES} ]]; then
    RELATED_IMAGES="${RELATED_IMAGE}"
  else
    RELATED_IMAGES="${RELATED_IMAGES}, ${RELATED_IMAGE}"
  fi

  sed -i -e "s;${source};${dest};" "${CSV_FILE_COPY}"
done

mv "${CSV_FILE_COPY}" "${CSV_FILE_COPY}.old"
yq -rY "
( .spec.relatedImages ) += [${RELATED_IMAGES}] |
( .spec.install.spec.deployments[0].spec.template.spec.containers[0].env ) += [${RELATED_IMAGES_ENV}]
" "${CSV_FILE_COPY}.old" > "${CSV_FILE_COPY}"
sed -i "${CSV_FILE_COPY}" -r -e "s|tag: |# tag: |"
rm -f "${CSV_FILE_COPY}.old"

# update original file with generated changes
mv "${CSV_FILE_COPY}" "${CSV_FILE}"
echo "[INFO] CSV updated: ${CSV_FILE}"


# cleanup
rm -fr "${BASE_DIR}/generated"
