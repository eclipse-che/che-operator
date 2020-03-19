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

usage () {
	echo "Usage:   $0 [-w WORKDIR] -c [/path/to/csv.yaml] "
	echo "Example: $0 -w $(pwd) -c  $(pwd)/generated/eclipse-che-preview-openshift/7.9.0/eclipse-che-preview-openshift.v7.9.0.clusterserviceversion.yaml"
}

if [[ $# -lt 1 ]]; then usage; exit; fi

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-w') BASE_DIR="$2"; shift 1;;
    '-c') CSV="$2"; shift 1;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! $CSV ]]; then usage; exit 1; fi

mkdir -p ${BASE_DIR}/generated

echo "[INFO] Get images from CSV ${CSV}"

IMAGE_LIST=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.name | test("IMAGE_default_.*"; "g")) | .value' "${CSV}")
OPERATOR_IMAGE=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].image' "${CSV}")

REGISTRY_LIST=$(yq -r '.spec.install.spec.deployments[].spec.template.spec.containers[].env[] | select(.name | test("IMAGE_default_.*_registry"; "g")) | .value' "${CSV}")
REGISTRY_IMAGES=""
for registry in ${REGISTRY_LIST}; do
  extracted=$(${SCRIPTS_DIR}/dockerContainerExtract.sh ${registry} var/www/html/*/external_images.txt | tail -n 1)

  # Container quay.io/eclipse/che-devfile-registry:7.9.0 unpacked to /tmp/quay.io-eclipse-che-devfile-registry-7.9.0-1584588272
  extracted=${extracted##* } # the last token in the above line is the path we want

  echo -n "[INFO] Extract images from registry ${registry} ... "
  if [[ -d ${extracted} ]]; then
    # cat ${extracted}/var/www/html/*/external_images.txt
    REGISTRY_IMAGES="${REGISTRY_IMAGES} $(cat ${extracted}/var/www/html/*/external_images.txt)"
  fi
  echo "found $(cat ${extracted}/var/www/html/*/external_images.txt | wc -l)"
  rm -fr ${extracted} 2>&1 >/dev/null
done

rm -Rf ${BASE_DIR}/generated/digests-mapping.txt
touch ${BASE_DIR}/generated/digests-mapping.txt
for image in ${OPERATOR_IMAGE} ${IMAGE_LIST} ${REGISTRY_IMAGES}; do
  case ${image} in
    *@sha256:*)
      withDigest="${image}";;
    *@)
      continue;;
    *)
      echo "[INFO] Get digest from ${image}"
      digest="$(skopeo inspect docker://${image} | jq -r '.Digest')"
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
