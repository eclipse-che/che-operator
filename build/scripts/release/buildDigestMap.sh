#!/bin/bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
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
SCRIPTS_DIR=$(dirname $(readlink -f "${BASH_SOURCE[0]}"))
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
command -v skopeo > /dev/null 2>&1 || { echo "skopeo is not installed. Aborting."; exit 1; }

excludedImages=(
                "quay.io/che-incubator/che-idea:next"
                "quay.io/che-incubator/che-idea-dev-server:next"
               )

usage () {
	echo "Usage:   $0 [-w WORKDIR] -c [/path/to/csv.yaml] -t [IMAGE_TAG]"
	echo "Example: $0 -w $(pwd) -c $(pwd)/bundle/next/eclipse-che/manifests/che-operator.clusterserviceversion.yaml -t 7.26.0"
}

setImagesFromDeploymentEnv() {
    REQUIRED_IMAGES=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.value) | select(.name | test("RELATED_IMAGE_.*"; "g")) | .value' "${CSV}" | sort | uniq)
}

setOperatorImage() {
    OPERATOR_IMAGE=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[0].image' "${CSV}")
}

writeDigest() {
  image=$1

  for i in "${excludedImages[@]}"; do
    if [[ ${i} =~ ${image} ]]; then
        echo "[INFO] Image '${image}' was excluded"
        return
    fi
  done

  imageType=$2
  digest=""
  case ${image} in
    *@sha256:*)
      withDigest=${image};;
    *@)
      return;;
    *)
      # for other build methods or for falling back to other registries when not found, can apply transforms here
      orig_image=${image}
      if [[ -x ${SCRIPTS_DIR}/buildDigestMapAlternateURLs.sh ]]; then
        # shellcheck source=buildDigestMapAlternateURLs.sh
        . ${SCRIPTS_DIR}/buildDigestMapAlternateURLs.sh
      fi
      if [[ ${digest} ]]; then
        if [[ ! "${QUIET}" ]]; then echo -n "[INFO] Got digest"; fi
        echo "    $digest \# ${image}"
      else
      image="${orig_image}"
      digest="$(skopeo inspect --tls-verify=false docker://${image} 2>/dev/null | jq -r '.Digest')"
      fi
      if [[ -z ${digest} ]]; then
        echo "[ERROR] Failed to get digest for image: ${image}"
        withoutTag=""
        withDigest=""
      else
        withoutTag="$(echo "${image}" | sed -e 's/^\(.*\):[^:]*$/\1/')"
        withDigest="${withoutTag}@${digest}";
      fi
  esac
  dots="${withDigest//[^\.]}"
  separators="${withDigest//[^\/]}"
  if [ "${#separators}" == "1" ] && [ "${#dots}" == "0" ]; then
    echo "[WARN] Add 'docker.io/' prefix to $image"
    withDigest="docker.io/${withDigest}"
  fi

  if [[ -n ${withDigest} ]]; then
    echo "${image}=${imageType}=${withDigest}" >> "${DIGEST_FILE}"
  fi
}

if [[ $# -lt 1 ]]; then usage; exit; fi

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-c') CSV="$2";shift 1;;
    '-t') IMAGE_TAG="$2"; shift 1;;
    '-q') QUIET="-q"; shift 0;;
    '--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! $CSV ]] || [[ ! $IMAGE_TAG ]]; then usage; exit 1; fi

mkdir -p "${SCRIPTS_DIR}/generated"

echo "[INFO] Get images from CSV: ${CSV}"

setImagesFromDeploymentEnv

setOperatorImage
echo "${OPERATOR_IMAGE}"

DIGEST_FILE=${SCRIPTS_DIR}/generated/digests-mapping.txt
rm -Rf "${DIGEST_FILE}"
touch "${DIGEST_FILE}"

writeDigest "${OPERATOR_IMAGE}" "operator-image"

for image in ${REQUIRED_IMAGES}; do
  writeDigest "${image}" "required-image"
done
