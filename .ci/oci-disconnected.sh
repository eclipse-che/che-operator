#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}"/.github/bin/oauth-provision.sh

# Define Disconnected tests environment
export INTERNAL_REGISTRY_URL=${INTERNAL_REGISTRY_URL-"UNDEFINED"}
export INTERNAL_REG_USERNAME=${INTERNAL_REG_USERNAME-"UNDEFINED"}
export INTERNAL_REG_PASS="${INTERNAL_REG_PASS-"UNDEFINED"}"
export SLACK_TOKEN="${SLACK_TOKEN-"UNDEFINED"}"
export WORKSPACE="${WORKSPACE-"UNDEFINED"}"
export REG_CREDS=${XDG_RUNTIME_DIR}/containers/auth.json
export ORGANIZATION="eclipse"
export TAG_NIGHTLY="nightly"

# catch and stop execution on any error
trap "catchDisconnectedJenkinsFinish" EXIT SIGINT

# Catch an error after existing from jenkins Workspace
function catchDisconnectedJenkinsFinish() {
    EXIT_CODE=$?

    if [ "$EXIT_CODE" != "0" ]; then
      export JOB_RESULT=":alert-siren: Failed :alert-siren:"
    else
      export JOB_RESULT=":tada: Success :tada:"
    fi

    mkdir -p ${WORKSPACE}/artifacts
    chectl server:logs --directory=${WORKSPACE}/artifacts

    echo "[INFO] Please check Jenkins Artifacts-> ${BUILD_URL}"
    /bin/bash "${OPERATOR_REPO}"/.github/bin/slack.sh

    exit $EXIT_CODE
}

# Check if all necessary environment for disconnected test are defined
if [[ "$WORKSPACE" == "UNDEFINED" ]]; then
    echo "[ERROR] Jenkins Workspace env is not defined."
    exit 1
fi

if [[ "$SLACK_TOKEN" == "UNDEFINED" ]]; then
    echo "[ERROR] Internal registry credentials environment is not defined."
    exit 1
fi

if [[ "$REG_CREDS" == "UNDEFINED" ]]; then
    echo "[ERROR] Internal registry credentials environment is not defined."
    exit 1
fi

if [[ "$INTERNAL_REGISTRY_URL" == "UNDEFINED" ]]; then
    echo "[ERROR] Internal registry url environment is not defined."
    exit 1
fi

if [[ "$INTERNAL_REG_USERNAME" == "UNDEFINED" ]]; then
    echo "[ERROR] Internal registry username environment is not defined."
    exit 1
fi

if [[ "$INTERNAL_REG_PASS" == "UNDEFINED" ]]; then
    echo "[ERROR] Internal registry password environment is not defined."
    exit 1
fi

# Login to internal registry using podman
podman login -u "${INTERNAL_REG_USERNAME}" -p "${INTERNAL_REG_PASS}" --tls-verify=false ${INTERNAL_REGISTRY_URL} --authfile=${REG_CREDS}

# Build che-plugin-registry and che-devfile-registry from Github Sources
# Che-Devfile-Registry Build
git clone git@github.com:eclipse/che-devfile-registry.git
cd che-devfile-registry

./build.sh --organization "${ORGANIZATION}" \
           --registry "${INTERNAL_REGISTRY_URL}" \
           --tag "${TAG_NIGHTLY}" \
           --offline
cd .. && rm -rf che-devfile-registry

# Che-Plugin-Registry-Build
git clone git@github.com:eclipse-che/che-plugin-registry.git
cd che-plugin-registry

export SKIP_TEST=true
./build.sh --organization "${ORGANIZATION}" \
           --registry "${INTERNAL_REGISTRY_URL}" \
           --tag "${TAG_NIGHTLY}" \
           --offline \
           --skip-digest-generation

cd .. && rm -rf che-plugin-registry

# Push devfile and plugins image to private registry
podman push --authfile="${REG_CREDS}" --tls-verify=false "${INTERNAL_REGISTRY_URL}"/"${ORGANIZATION}"/che-devfile-registry:"${TAG_NIGHTLY}"
podman push --authfile="${REG_CREDS}" --tls-verify=false "${INTERNAL_REGISTRY_URL}"/"${ORGANIZATION}"/che-plugin-registry:"${TAG_NIGHTLY}"

# Get all containers images used in eclipse-che deployment(postgresql, che-server, che-dashboard, keycloak...)
curl -sSLo- https://raw.githubusercontent.com/eclipse-che/che-operator/main/deploy/operator.yaml > /tmp/yam.yaml
export ARRAY_OF_IMAGES=$(cat /tmp/yam.yaml | yq '.spec.template.spec.containers[0].env[] | select(.name|test("RELATED_")) | .value' -r)

# Remove from Array of images devfile and plugins because will be builded using build.sh in offline mode.
for delete in 'quay.io/eclipse/che-plugin-registry:nightly' 'quay.io/eclipse/che-devfile-registry:nightly'
do
    #Quotes when working with strings
    ARRAY_OF_IMAGES=("${ARRAY_OF_IMAGES[@]/$delete}")
done

# Copy all che components to internal registry
for IMAGE in ${ARRAY_OF_IMAGES[@]};
do
    echo -e "[INFO] Copying image ${IMAGE} to internal registry..."
    if [[ "$IMAGE" =~ ^registry.access.redhat.com* ]]; then
        IMG_VALUE=$(echo $IMAGE | sed -e "s/registry.access.redhat.com/""/g")
        sudo skopeo copy --authfile=${REG_CREDS} --dest-tls-verify=false docker://"${IMAGE}" docker://"${INTERNAL_REGISTRY_URL}/eclipse${IMG_VALUE}"
    fi

    if [[ "$IMAGE" =~ ^quay.io* ]]; then
        IMG_VALUE=$(echo $IMAGE | sed -e "s/quay.io/"${INTERNAL_REGISTRY_URL}"/g")
        sudo skopeo copy --authfile=${REG_CREDS} --dest-tls-verify=false docker://"${IMAGE}" docker://"${IMG_VALUE}"
    fi
done

# Copy Che Operator into private registry
sudo skopeo copy --authfile=${REG_CREDS} --dest-tls-verify=false docker://quay.io/eclipse/che-operator:nightly docker://${INTERNAL_REGISTRY_URL}/eclipse/che-operator:nightly

# Filter all necessary plugins need it to start a workspace in disconnected en
IFS=$'\r\n' GLOBIGNORE='*' command eval 'PLUGINS_IMAGES=($(podman run --authfile=${XDG_RUNTIME_DIR}/containers/auth.json -it --rm \
  --entrypoint cat ${INTERNAL_REGISTRY_URL}/eclipse/che-plugin-registry:nightly  /var/www/html/v3/external_images.txt))'
for container in "${PLUGINS_IMAGES[@]}";
do
    if [[ $container != *"che-plugin-sidecar"* ]] &&
       [[ $container != *"che-editor"* ]] && \
       [[ $container != *"codercom"* ]] && \
       [[ $container != "docker.io"* ]]; then
        REGISTRY_IMG_NAME=$(echo $container | sed -e "s/quay.io/"${INTERNAL_REGISTRY_URL}"/g")
        sudo skopeo copy --authfile=${REG_CREDS} --dest-tls-verify=false docker://"${container}" docker://"${REGISTRY_IMG_NAME}"
    fi
done

# Obtain workspace golang SIDECAR_IMAGE and copy to internal registry
podman run --authfile=${XDG_RUNTIME_DIR}/containers/auth.json -it --rm \
  --entrypoint cat "${INTERNAL_REGISTRY_URL}"/eclipse/che-plugin-registry:nightly  /var/www/html/v3/plugins/golang/go/latest/meta.yaml > /tmp/workspace.yaml

export SIDECAR_IMAGE=$(cat /tmp/workspace.yaml | yq '.spec.containers[] | .image' -r)
if [[ "$SIDECAR_IMAGE" =~ ^quay.io* ]]; then
    REGISTRY_IMG_NAME=$(echo $SIDECAR_IMAGE | sed -e "s/quay.io/"${INTERNAL_REGISTRY_URL}"/g")
    sudo skopeo copy --authfile=${REG_CREDS} --dest-tls-verify=false docker://"${SIDECAR_IMAGE}" docker://"${REGISTRY_IMG_NAME}"
fi

# Obtain the golang image and push to internal Registry
IFS=$'\r\n' GLOBIGNORE='*' command eval 'DEVFILE_IMAGES=($(podman run --authfile=${XDG_RUNTIME_DIR}/containers/auth.json -it --rm \
  --entrypoint cat ${INTERNAL_REGISTRY_URL}/eclipse/che-devfile-registry:nightly /var/www/html/devfiles/external_images.txt))'

for container in "${DEVFILE_IMAGES[@]}"
do
    if [[ $container == *"che-golang"* ]]; then
        REGISTRY_IMG_NAME=$(echo $container | sed -e "s/quay.io/"${INTERNAL_REGISTRY_URL}"/g")
        sudo skopeo copy --authfile=${REG_CREDS} --dest-tls-verify=false docker://"${container}" docker://"${REGISTRY_IMG_NAME}"
    fi
done

# Get the ocp domain for che custom resources
export DOMAIN=$(oc get dns cluster -o json | jq .spec.baseDomain | sed -e 's/^"//' -e 's/"$//')

# Define the CR patch specifying the airgap registry and nonProxy-hosts
cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  auth:
    updateAdminPassword: false
  server:
    airGapContainerRegistryHostname: $INTERNAL_REGISTRY_URL
    airGapContainerRegistryOrganization: 'eclipse'
    nonProxyHosts: oauth-openshift.apps.$DOMAIN
EOL

# Provision test user to openshift cluster
initDefaults
provisionOpenShiftOAuthUser

# Deploy Eclipse Che and retrieve golang devfile from devfile-registry
chectl server:deploy --telemetry=off --k8spodwaittimeout=1800000 --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml --che-operator-image=${INTERNAL_REGISTRY_URL}/eclipse/che-operator:nightly --platform=openshift --installer=operator
DEVFILEURL=$(oc get checluster/eclipse-che -n eclipse-che -o "jsonpath={.status.devfileRegistryURL}")
curl -sSLo- -vk "${DEVFILEURL}/devfiles/go/devfile.yaml" > /tmp/devfile.yaml

# Link openshift User with keycloak and start golang workspace
provisionOAuth
chectl auth:login -u admin -p admin
chectl workspace:create --start --devfile=/tmp/devfile.yaml
waitWorkspaceStart
