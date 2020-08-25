#!/bin/bash

IMAGE_REGISTRY_HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
NAMESPACE=che
CATALOG_BUNDLE_IMAGE_NAME="che_operator_bundle-openshift:0.0.1"

CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_BUNDLE_IMAGE_NAME}"

HOME="/home/user"
OPM_BINARY="${HOME}/projects/operator-registry/bin/opm"

if [ -z "${CATALOG_SOURCE_IMAGE_NAME}" ]; then
    CATALOG_SOURCE_IMAGE_NAME="operator-catalog-source-openshift:0.0.1"
fi

if [ -z "${CATALOG_SOURCE_IMAGE}" ]; then
    CATALOG_SOURCE_IMAGE="${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_SOURCE_IMAGE_NAME}"  
fi

echo "=================Build catalog source"
# sudo cp -rf /home/user/crt/test.crt /usr/share/pki/ca-trust-source/anchors/ && update-ca-trust

CA_PATH="${HOME}/crt/test.crt"
# sudo mkdir -p "/etc/containers/cert.d/${IMAGE_REGISTRY_HOST}"
# sudo cp -rf "${CA_PATH}" "/etc/containers/cert.d/${IMAGE_REGISTRY_HOST}/"
# sudo systemctl restart podman

echo "try to pull image..."
# podman login -u kubeadmin -p $(oc whoami -t) "${IMAGE_REGISTRY_HOST}" --cert-dir "${HOME}/crt"
# podman pull "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"
echo "Pull done"

${OPM_BINARY} index add \
    --bundles "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --tag "${CATALOG_SOURCE_IMAGE}" \
    --mode semver \
    --generate \
    --pull-tool "podman" \
    --ca-file "${CA_PATH}" \
    --debug \
    --skip-tls
    # 
    # --skip-tls \
    # --build-tool "${imageTool}" 
    # --pull-tool "${imageTool}"
    # --container-tool

    # --permissive
    # --out-dockerfile "/home/user/GoWorkSpace/src/github.com/eclipse/che-operator/olm/test/Dockerfile"
