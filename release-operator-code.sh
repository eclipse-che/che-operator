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

set -e

REGEX="^([0-9]+)\\.([0-9]+)\\.([0-9]+)(\\-[0-9a-z-]+(\\.[0-9a-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
if [[ "$1" =~ $REGEX ]]
then
  RELEASE="$1"
else
  echo "You should provide the new release as the first parameter"
  echo "and it should be semver-compatible with optional *lower-case* pre-release part"
  exit 1
fi

cd "${BASE_DIR}"

echo
echo "## Creating release '${RELEASE}' of the Che operator docker image"

lastDefaultCheVersion=$(grep -e '^[^a-zA-Z]*defaultCheServerImageTag' "pkg/deploy/defaults.go" | sed -e 's/^[^a-zA-Z]*defaultCheServerImageTag *= *"\([^"]*\)"/\1/')
lastDefaultKeycloakVersion=$(grep -e '^[^a-zA-Z]*defaultKeycloakUpstreamImage' "pkg/deploy/defaults.go" | sed -e 's/^[^a-zA-Z]*defaultKeycloakUpstreamImage *= *"[^":]*:\([^"]*\)"/\1/')
lastDefaultPluginRegistryVersion=$(grep -e '^[^a-zA-Z]*defaultPluginRegistryUpstreamImage' "pkg/deploy/defaults.go" | sed -e 's/^[^a-zA-Z]*defaultPluginRegistryUpstreamImage *= *"[^":]*:\([^"]*\)"/\1/')
lastDefaultDevfileRegistryVersion=$(grep -e '^[^a-zA-Z]*defaultDevfileRegistryUpstreamImage' "pkg/deploy/defaults.go" | sed -e 's/^[^a-zA-Z]*defaultDevfileRegistryUpstreamImage *= *"[^":]*:\([^"]*\)"/\1/')
if [ "${lastDefaultCheVersion}" != "${lastDefaultKeycloakVersion}" ]
then
  echo "#### ERROR ####"
  echo "Current default Che version: ${lastDefaultCheVersion}"
  echo "Current default Keycloak version: ${lastDefaultKeycloakVersion}"
  echo "Current default Devfile Registry version: ${lastDefaultDevfileRegistryVersion}"
  echo "Current default Plugin Registry version: ${lastDefaultPluginRegistryVersion}"
  echo "Current default version for various Che containers are not the same in file 'pkg/deploy/defaults.go'."
  echo "Please fix that manually first !"
  exit 1
fi

lastDefaultVersion="${lastDefaultCheVersion}"
echo "   - Current default version of Che containers: ${lastDefaultVersion}"
echo "   - New version to apply as default version for Che containers: ${RELEASE}"
if [ "${lastDefaultVersion}" == "${RELEASE}" ]
then
  echo "Release ${RELEASE} already exists as the default in the Operator Go code !"
  exit 1
fi

echo "     => will update default Eclipse Che Keycloak docker image tags from '${lastDefaultVersion}' to '${RELEASE}'"

sed \
-e "s/\(.*defaultCheServerImageTag *= *\"\)[^\"]*\"/\1${RELEASE}\"/" \
-e "s/\(.*defaultKeycloakUpstreamImage *= *\"[^\":]*:\)[^\"]*\"/\1${RELEASE}\"/" \
-e "s/\(.*defaultPluginRegistryUpstreamImage *= *\"[^\":]*:\)[^\"]*\"/\1${RELEASE}\"/" \
-e "s/\(.*defaultDevfileRegistryUpstreamImage *= *\"[^\":]*:\)[^\"]*\"/\1${RELEASE}\"/" \
pkg/deploy/defaults.go \
> pkg/deploy/defaults.go.new
mv pkg/deploy/defaults.go.new pkg/deploy/defaults.go

dockerImage="quay.io/eclipse/che-operator:${RELEASE}"
echo "   - Building Che Operator docker image for new release ${RELEASE}"
docker build -t "quay.io/eclipse/che-operator:${RELEASE}" .

echo
echo "## Released Che operator docker image: ${dockerImage}"
echo "## Now you will probably want to:"
echo "      - Push your '${dockerImage}' docker image"
echo "      - Release the Che operator OLM packages with command:"
echo "        ./olm/release-olm-files.sh ${RELEASE}"
echo "      - Commit your changes"
echo "      - Push the the Che operator OLM packages to Quay.io preview applications with command:"
echo "        ./olm/push-olm-files-to-quay.sh ${RELEASE}"
cd "${CURRENT_DIR}"
