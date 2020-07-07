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
command -v skopeo > /dev/null 2>&1 || { echo "skopeo is not installed. Aborting."; exit 1; }

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

source ${BASE_DIR}/images.sh

# todo create init method
setImagesFromDeploymentEnv

setOperatorImage
echo ${OPERATOR_IMAGE}

setPluginRegistryList
echo ${PLUGIN_REGISTRY_LIST}

setDevfileRegistryList
echo ${DEVFILE_REGISTRY_LIST}

writeDigest() {
  image=$1
  imageType=$2
  echo ${image}
  case ${image} in
    *@sha256:*)
      withDigest="${image}";;
    *@)
      continue;;
    *)
      digest="$(skopeo inspect --tls-verify=false docker://${image} 2>/dev/null | jq -r '.Digest')"
      if [[ ${digest} ]]; then
        if [[ ! "${QUIET}" ]]; then echo -n "[INFO] Got digest"; fi
        echo "    $digest # ${image}"
      else
        # for other build methods or for falling back to other registries when not found, can apply transforms here
        if [[ -x ${BASE_DIR}/buildDigestMapAlternateURLs.sh ]]; then
          . ${BASE_DIR}/buildDigestMapAlternateURLs.sh
        fi
      fi
      if [[ -z ${digest} ]]; then
        echo "==================== Failed to get digest for image: ${image}======================"
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
    echo "${image}=${imageType}=${withDigest}" >> ${DIGEST_FILE}
  fi
}

DIGEST_FILE=${BASE_DIR}/generated/digests-mapping.txt
rm -Rf ${DIGEST_FILE}
touch ${DIGEST_FILE}

writeDigest ${OPERATOR_IMAGE} "operator-image"

for image in ${REQUIRED_IMAGES}; do
  writeDigest ${image} "required-image" 
done

for image in ${PLUGIN_REGISTRY_LIST}; do
  writeDigest ${image} "plugin-registry-image"
done

for image in ${DEVFILE_REGISTRY_LIST}; do
  writeDigest ${image} "devfile-registry-image"
done
