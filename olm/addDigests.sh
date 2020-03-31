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
BASE_DIR="$(pwd)"
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
	echo "Usage:   $0 [-w WORKDIR] -s [SOURCE_PATH] -n [csv name] -v [VERSION] "
	echo "Example: $0 -w $(pwd) -s eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift -n eclipse-che-preview-openshift -v 7.9.0"
	echo "Example: $0 -w $(pwd) -s controller-manifests -n codeready-workspaces -v 2.1.0"
}

if [[ $# -lt 1 ]]; then usage; exit; fi

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-w') BASE_DIR="$2"; shift 1;;
    '-s') SRC_DIR="$2"; shift 1;;
    '-n') CSV_NAME="$2"; shift 1;;
    '-v') VERSION="$2"; shift 1;;
    '-q') QUIET="-q"; shift 0;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! $SRC_DIR ]] || [[ ! $CSV_NAME ]] || [[ ! $VERSION ]]; then usage; exit 1; fi

rm -Rf ${BASE_DIR}/generated/${CSV_NAME}/
mkdir -p ${BASE_DIR}/generated/${CSV_NAME}/
cp -R ${BASE_DIR}/${SRC_DIR}/* ${BASE_DIR}/generated/${CSV_NAME}/

CSV_FILE="$(find ${BASE_DIR}/generated/${CSV_NAME}/*${VERSION}/ -name "${CSV_NAME}.*${VERSION}.clusterserviceversion.yaml" | tail -1)"; # echo "[INFO] CSV = ${CSV_FILE}"
${SCRIPTS_DIR}/buildDigestMap.sh -w ${BASE_DIR} -c ${CSV_FILE} -v ${VERSION} ${QUIET}

# inject relatedImages block
names=" "
count=1
RELATED_IMAGES='. * { spec : { relatedImages: [ '
if [[ ! "${QUIET}" ]]; then cat ${BASE_DIR}/generated/digests-mapping.txt; fi
for mapping in $(cat ${BASE_DIR}/generated/digests-mapping.txt)
do
  source=$(echo "${mapping}" | sed -e 's/\(.*\)=.*/\1/')
  dest=$(echo "${mapping}" | sed -e 's/.*=\(.*\)/\1/')
  sed -i -e "s;${source};${dest};" ${CSV_FILE}
  name=$(echo "${dest}" | sed -e 's;.*/\([^\/][^\/]*\)@.*;\1;')
  nameWithSpaces=" ${name} "
  if [[ "${names}" != *${nameWithSpaces}* ]]; then
    if [ "${names}" != " " ]; then
      RELATED_IMAGES="${RELATED_IMAGES},"
    fi
    RELATED_IMAGES="${RELATED_IMAGES} { name: \"${name}\", image: \"${dest}\", tag: \"${source}\"}"
    names="${names} ${name} "
  fi
done
RELATED_IMAGES="${RELATED_IMAGES} ] } }"
mv ${CSV_FILE} ${CSV_FILE}.old
yq -Y "$RELATED_IMAGES" ${CSV_FILE}.old > ${CSV_FILE}
sed -i ${CSV_FILE} -r -e "s|tag: |# tag: |" 
rm -f ${CSV_FILE}.old

# update original file with generated changes
CSV_FILE_ORIG=$(find ${BASE_DIR} -name "${CSV_FILE##*/}" | grep -v generated | tail -1)
mv "${CSV_FILE}" "${CSV_FILE_ORIG}"
echo "[INFO] CSV updated: ${CSV_FILE_ORIG}"

# cleanup
rm -fr ${BASE_DIR}/generated