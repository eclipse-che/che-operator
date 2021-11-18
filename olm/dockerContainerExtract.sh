#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

set -e +x

if [[ ! $1 ]]; then
  echo "Usage: $0 CONTAINER [tar-extraction-flags]"
  echo "Usage: $0 quay.io/crw/operator-metadata:latest"
  echo "Usage: $0 quay.io/crw/devfileregistry-rhel8:latest var/www/html/*/external_images.txt"
  echo "Usage: $0 quay.io/crw/pluginregistry-rhel8:latest var/www/html/*/external_images.txt"
  exit
fi

PODMAN=$(command -v podman)
if [[ ! -x $PODMAN ]]; then
  echo "[WARNING] podman is not installed."
  PODMAN=$(command -v docker)
  if [[ ! -x $PODMAN ]]; then
    echo "[ERROR] docker is not installed. Aborting."; exit 1
  fi
fi

container="$1"; shift 1
tmpcontainer="$(echo $container | tr "/:" "--")-$(date +%s)"
unpackdir="/tmp/${tmpcontainer}"

# get remote image
echo "[INFO] Pulling $container ..."
${PODMAN} pull $container 2>&1

# create local container
${PODMAN} rm -f "${tmpcontainer}" 2>&1 >/dev/null || true
# use sh for regular containers or ls for scratch containers
${PODMAN} create --name="${tmpcontainer}" $container sh 2>&1 >/dev/null || ${PODMAN} create --name="${tmpcontainer}" $container ls 2>&1 >/dev/null 

# export and unpack
${PODMAN} export "${tmpcontainer}" > /tmp/${tmpcontainer}.tar
rm -fr "$unpackdir"; mkdir -p "$unpackdir"
echo "[INFO] Extract from container ..."
tar xf /tmp/${tmpcontainer}.tar --wildcards -C "$unpackdir" $*

# cleanup
${PODMAN} rm -f "${tmpcontainer}" 2>&1 >/dev/null || true
rm -fr  /tmp/${tmpcontainer}.tar

echo "[INFO] Container $container unpacked to $unpackdir"
