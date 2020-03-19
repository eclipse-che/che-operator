#!/bin/bash
set -e +x

if [[ ! $1 ]]; then
  echo "Usage: $0 CONTAINER [tar-extraction-flags]"
  echo "Usage: $0 quay.io/crw/operator-metadata:latest"
  echo "Usage: $0 quay.io/crw/devfileregistry-rhel8:latest var/www/html/*/external_images.txt"
  echo "Usage: $0 quay.io/crw/pluginregistry-rhel8:latest var/www/html/*/external_images.txt"
  exit
fi

PODMAN=docker # or user podman

container="$1"; shift 1
tmpcontainer="$(echo $container | tr "/:" "--")-$(date +%s)"
unpackdir="/tmp/${tmpcontainer}"

# get remote image
echo "[INFO] Pulling $container ..."
${PODMAN} pull $container 2>&1 >/dev/null

# create local container
${PODMAN} rm -f "${tmpcontainer}" 2>&1 >/dev/null || true
# use sh for regular containers or ls for scratch containers
${PODMAN} create --name="${tmpcontainer}" $container sh 2>&1 >/dev/null || ${PODMAN} create --name="${tmpcontainer}" $container ls 2>&1 >/dev/null 

# export and unpack
${PODMAN} export "${tmpcontainer}" > /tmp/${tmpcontainer}.tar
rm -fr "$unpackdir"; mkdir -p "$unpackdir"
echo "[INFO] Extract from container ..."
tar xf /tmp/${tmpcontainer}.tar -C "$unpackdir" $*

# cleanup
${PODMAN} rm -f "${tmpcontainer}" 2>&1 >/dev/null || true
rm -fr  /tmp/${tmpcontainer}.tar

echo "[INFO] Container $container unpacked to $unpackdir"
