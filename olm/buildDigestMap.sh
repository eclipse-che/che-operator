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

SCRIPTS_DIR=$(cd "$(dirname "$0")"; pwd)
BASE_DIR="$1"
QUIET=""

PODMAN=$(command -v podman)
if [[ ! -x $PODMAN ]]; then
  echo "[WARNING] podman is not installed."
  PODMAN=$(command -v docker)
  if [[ ! -x $PODMAN ]]; then
    echo "[ERROR] docker is not installed. Aborting."; exit 1
  fi
fi
command -v yq >/dev/null 2>&1 || { echo "yq is not installed. Aborting."; exit 1; }

usage () {
	echo "Usage:   $0 [-w WORKDIR] -c [/path/to/csv.yaml] "
	echo "Example: $0 -w $(pwd) -c  $(pwd)/generated/eclipse-che-preview-openshift/7.9.0/eclipse-che-preview-openshift.v7.9.0.clusterserviceversion.yaml"
}

if [[ $# -lt 1 ]]; then usage; exit; fi

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-w') BASE_DIR="$2"; shift 1;;
    '-c') CSV="$2"; shift 1;;
    '-v') VERSION="$2"; shift 1;;
    '-q') QUIET="-q"; shift 0;;
    '--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! $CSV ]] || [[ ! $VERSION ]]; then usage; exit 1; fi

mkdir -p ${BASE_DIR}/generated

echo "[INFO] Get images from CSV ${CSV}"

IMAGE_LIST=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.name | test("IMAGE_default_.*"; "g")) | .value' "${CSV}")
OPERATOR_IMAGE=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].image' "${CSV}")

REGISTRY_LIST=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.name | test("IMAGE_default_.*_registry"; "g")) | .value' "${CSV}")
REGISTRY_IMAGES_ALL=""
for registry in ${REGISTRY_LIST}; do
  registry="${registry/\@sha256:*/:${VERSION}}" # remove possible existing @sha256:... and use current version instead
  # echo -n "[INFO] Pull container ${registry} ..."
  ${PODMAN} pull ${registry} ${QUIET}

  REGISTRY_IMAGES="$(${PODMAN} run --rm  --entrypoint /bin/sh  ${registry} -c "cat /var/www/html/*/external_images.txt")"
  echo "[INFO] Found $(echo "${REGISTRY_IMAGES}" | wc -l) images in registry"
  REGISTRY_IMAGES_ALL="${REGISTRY_IMAGES_ALL} ${REGISTRY_IMAGES}"
done

rm -Rf ${BASE_DIR}/generated/digests-mapping.txt
touch ${BASE_DIR}/generated/digests-mapping.txt
for image in ${OPERATOR_IMAGE} ${IMAGE_LIST} ${REGISTRY_IMAGES_ALL}; do
  case ${image} in
    *@sha256:*)
      withDigest="${image}";;
    *@)
      continue;;
    *)
      digest="$(skopeo inspect docker://${image} 2>/dev/null | jq -r '.Digest')"
      if [[ ${digest} ]]; then
        if [[ ! "${QUIET}" ]]; then echo -n "[INFO] Got digest"; fi
        echo "    $digest # ${image}"
      else
        # for other build methods or for falling back to other registries when not found, can apply transforms here
        if [[ -x ${SCRIPTS_DIR}/buildDigestMapAlternateURLs.sh ]]; then
          . ${SCRIPTS_DIR}/buildDigestMapAlternateURLs.sh
        fi
      fi
      withoutTag="$(echo "${image}" | sed -e 's/^\(.*\):[^:]*$/\1/')"
      withDigest="${withoutTag}@${digest}";;
  esac
  dots="${withDigest//[^\.]}"
  separators="${withDigest//[^\/]}"
  if [ "${#separators}" == "1" ] && [ "${#dots}" == "0" ]; then
    echo "[WARN] Add 'docker.io/' prefix to $image"
    withDigest="docker.io/${withDigest}"
  fi

  echo "${image}=${withDigest}" >> ${BASE_DIR}/generated/digests-mapping.txt
done
