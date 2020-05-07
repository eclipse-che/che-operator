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
UBI8_MINIMAL_IMAGE=$2

cd "${BASE_DIR}"

echo
echo "## Creating release '${RELEASE}' of the Che operator docker image"

OPERATOR_YAML="${BASE_DIR}"/deploy/operator.yaml
OPERATOR_LOCAL_YAML="${BASE_DIR}"/deploy/operator-local.yaml

lastDefaultCheVersion=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"CHE_VERSION\") | .value" "${OPERATOR_YAML}")

lastDefaultCheServerImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"IMAGE_default_che_server\") | .value" "${OPERATOR_YAML}")

lastDefaultKeycloakImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"IMAGE_default_keycloak\") | .value" "${OPERATOR_YAML}")
lastDefaultKeycloakVersion=$(echo ${lastDefaultKeycloakImage} | sed -e 's/^[^a-zA-Z]*[^:]*:\([^"]*\)/\1/')

lastDefaultPluginRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"IMAGE_default_plugin_registry\") | .value" "${OPERATOR_YAML}")
lastDefaultPluginRegistryVersion=$(echo ${lastDefaultPluginRegistryImage} | sed -e 's/^[^a-zA-Z]*[^:]*:\([^"]*\)/\1/')

lastDefaultDevfileRegistryImage=$(yq -r ".spec.template.spec.containers[] | select(.name == \"che-operator\") | .env[] | select(.name == \"IMAGE_default_devfile_registry\") | .value" "${OPERATOR_YAML}")
lastDefaultDevfileRegistryVersion=$(echo ${lastDefaultDevfileRegistryImage} | sed -e 's/^[^a-zA-Z]*[^:]*:\([^"]*\)/\1/')

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

function replaceImageTag() {
    echo "${1}" | sed -e "s/\(.*:\).*/\1${2}/"
}

wget https://raw.githubusercontent.com/eclipse/che/${RELEASE}/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties
PLUGIN_BROKER_METADATA_IMAGE_RELEASE=$(cat /tmp/che.properties| grep "che.workspace.plugin_broker.metadata.image" | cut -d = -f2)
PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE=$(cat /tmp/che.properties | grep "che.workspace.plugin_broker.artifacts.image" | cut -d = -f2)
JWT_PROXY_IMAGE_RELEASE=$(cat /tmp/che.properties | grep "che.server.secure_exposer.jwtproxy.image" | cut -d = -f2)
CHE_SERVER_IMAGE_REALEASE=$(replaceImageTag "${lastDefaultCheServerImage}" "${RELEASE}")
KEYCLOAK_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultKeycloakImage}" "${RELEASE}")
PLUGIN_REGISTRY_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultPluginRegistryImage}" "${RELEASE}")
DEVFILE_REGISTRY_IMAGE_RELEASE=$(replaceImageTag "${lastDefaultDevfileRegistryImage}" "${RELEASE}")
rm /tmp/che.properties

NEW_OPERATOR_YAML="${OPERATOR_YAML}.new"
NEW_OPERATOR_LOCAL_YAML="${OPERATOR_LOCAL_YAML}.new"
# copy licence header
eval head -10 "${OPERATOR_YAML}" > ${NEW_OPERATOR_YAML}
eval head -10 "${OPERATOR_LOCAL_YAML}" > ${NEW_OPERATOR_LOCAL_YAML}

cat "${OPERATOR_YAML}" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\") | .image ) = \"quay.io/eclipse/che-operator:${RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"CHE_VERSION\") | .value ) = \"${RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_server\") | .value ) = \"${CHE_SERVER_IMAGE_REALEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_keycloak\") | .value ) = \"${KEYCLOAK_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_plugin_registry\") | .value ) = \"${PLUGIN_REGISTRY_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_devfile_registry\") | .value ) = \"${DEVFILE_REGISTRY_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_pvc_jobs\") | .value ) = \"${UBI8_MINIMAL_IMAGE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_workspace_plugin_broker_metadata\") | .value ) = \"${PLUGIN_BROKER_METADATA_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_workspace_plugin_broker_artifacts\") | .value ) = \"${PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"${JWT_PROXY_IMAGE_RELEASE}\"" \
>> "${NEW_OPERATOR_YAML}"
mv "${NEW_OPERATOR_YAML}" "${OPERATOR_YAML}"

cat "${OPERATOR_LOCAL_YAML}" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\") | .image ) = \"quay.io/eclipse/che-operator:${RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"CHE_VERSION\") | .value ) = \"${RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_server\") | .value ) = \"${CHE_SERVER_IMAGE_REALEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_keycloak\") | .value ) = \"${KEYCLOAK_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_plugin_registry\") | .value ) = \"${PLUGIN_REGISTRY_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_devfile_registry\") | .value ) = \"${DEVFILE_REGISTRY_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_pvc_jobs\") | .value ) = \"${UBI8_MINIMAL_IMAGE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_workspace_plugin_broker_metadata\") | .value ) = \"${PLUGIN_BROKER_METADATA_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_workspace_plugin_broker_artifacts\") | .value ) = \"${PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"IMAGE_default_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"${JWT_PROXY_IMAGE_RELEASE}\"" \
>> "${NEW_OPERATOR_LOCAL_YAML}"
mv "${NEW_OPERATOR_LOCAL_YAML}" "${OPERATOR_LOCAL_YAML}"

defaulTest=${BASE_DIR}/pkg/deploy/defaults_test.go
sed -i 's|cheVersionTest           = ".*"|cheVersionTest           = "'${RELEASE}'"|g'  $defaulTest
sed -i 's|cheServerImageTest       = ".*"|cheServerImageTest       = "'"$CHE_SERVER_IMAGE_REALEASE"'"|g'  $defaulTest
sed -i 's|cheOperatorImageTest     = ".*"|cheOperatorImageTest     = "'"quay.io/eclipse/che-operator:${RELEASE}"'"|g'  $defaulTest
sed -i 's|pluginRegistryImageTest  = ".*"|pluginRegistryImageTest  = "'${PLUGIN_REGISTRY_IMAGE_RELEASE}'"|g'  $defaulTest
sed -i 's|devfileRegistryImageTest = ".*"|devfileRegistryImageTest = "'${DEVFILE_REGISTRY_IMAGE_RELEASE}'"|g'  $defaulTest
sed -i 's|pvcJobsImageTest         = ".*"|pvcJobsImageTest         = "'${UBI8_MINIMAL_IMAGE}'"|g'  $defaulTest
sed -i 's|keycloakImageTest        = ".*"|keycloakImageTest        = "'${KEYCLOAK_IMAGE_RELEASE}'"|g'  $defaulTest
sed -i 's|brokerMetadataTest       = ".*"|brokerMetadataTest       = "'${PLUGIN_BROKER_METADATA_IMAGE_RELEASE}'"|g'  $defaulTest
sed -i 's|brokerArtifactsTest      = ".*"|brokerArtifactsTest      = "'${PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE}'"|g'  $defaulTest
sed -i 's|jwtProxyTest             = ".*"|jwtProxyTest             = "'${JWT_PROXY_IMAGE_RELEASE}'"|g'  $defaulTest

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
