#!/bin/bash

SCRIPT=$(readlink -f "$0")
export SCRIPT

BASE_DIR=$(dirname "$(dirname "$SCRIPT")");
export BASE_DIR

namespace=che

if [ ! $(oc get configs.imageregistry.operator.openshift.io/cluster -o yaml | yq -r ".spec.defaultRoute") == true ];then
    oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
fi

setUpCrt() {
    HOST=$(oc get route default-route -n openshift-image-registry -o yaml | yq -r ".spec.host")
    certBundle=$(echo "Q" | openssl s_client -showcerts -connect "${HOST}":443)
    CA_CRT="${HOME}/crt/test.crt"
    rm -rf "${CA_CRT}"
    touch "${CA_CRT}"

    echo "${certBundle}" |
    while IFS= read -r line
    do
    if [ "${line}" == "-----BEGIN CERTIFICATE-----" ]; then
        IS_CERT_STARTED=true
    fi
    
    if [ "${IS_CERT_STARTED}" == true ]; then
        CERT="${CERT}${line}\n"
    fi

    if [ "${line}" == "-----END CERTIFICATE-----" ]; then
        if echo -e "${CERT}" | openssl x509 -text | grep -q "CA:TRUE"; then
            echo "CA sertificate found! And store by path ${CA_CRT}"
            echo -e "${CERT}" > "${CA_CRT}"
            exit 0
        fi
        CERT=""
        IS_CERT_STARTED="false"
    fi
    done


    oc create configmap user-ca-bundle --from-file=ca-bundle.crt="${CA_CRT}"  -n openshift-config
    oc get configmap user-ca-bundle  -n openshift-config -o yaml
    oc patch proxy/cluster --patch '{"spec":{"trustedCA":{"name":"user-ca-bundle"}}}' --type=merge
    oc create configmap all-ca
    oc label configmap all-ca config.openshift.io/inject-trusted-cabundle=true
}
setUpCrt

platform=openshift
OPM_BUNDLE_DIR="${BASE_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"

pushd "${OPM_BUNDLE_DIR}" || exit

echo "[INFO] build bundle image for dir: ${OPM_BUNDLE_DIR}"

imageTool="podman"
OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
CATALOG_BUNDLE_IMAGE_NAME="che_operator_bundle-openshift:0.0.1"
 
IMAGE_REGISTRY_HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
${imageTool} logout "https://${IMAGE_REGISTRY_HOST}"
${imageTool} login -u kubeadmin -p $(oc whoami -t) "${IMAGE_REGISTRY_HOST}" --cert-dir "${HOME}/crt"
# --tls-verify=false

# IMAGE_REGISTRY_HOST=quay.io
# NAMESPACE=aandriienko

NAMESPACE=che

oc new-project "${namespace}"
CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_BUNDLE_IMAGE_NAME}"
OPM_BINARY="${HOME}/projects/operator-registry/bin/opm"
# OPM_BINARY=opm

${OPM_BINARY} alpha bundle build \
    -d "${OPM_BUNDLE_MANIFESTS_DIR}" \
    --tag "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --package "eclipse-che-preview-openshift" \
    --channels "stable,nightly" \
    --default "stable" \
    --image-builder "${imageTool}"

# ${OPM_BINARY} alpha bundle validate -t "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" --image-builder "${imageTool}"

${imageTool} push "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" --cert-dir "${HOME}/crt"
# --tls-verify=false

echo "=================Build catalog source"
if [ -z "${CATALOG_SOURCE_IMAGE_NAME}" ]; then
    CATALOG_SOURCE_IMAGE_NAME="operator-catalog-source-openshift:0.0.1"
fi

if [ -z "${CATALOG_SOURCE_IMAGE}" ]; then
    CATALOG_SOURCE_IMAGE="${IMAGE_REGISTRY_HOST}/${NAMESPACE}/${CATALOG_SOURCE_IMAGE_NAME}"  
fi

pushd "${OPM_BUNDLE_DIR}" || true

${OPM_BINARY} index add \
    --bundles "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --tag "${CATALOG_SOURCE_IMAGE}" \
    --mode semver \
    --skip-tls \
    --pull-tool "podman"

# --generate \

# --ca-file "/home/user/crt/test.crt" \# --build-tool "${imageTool}" 
# --pull-tool "${imageTool}"
# --container-tool

# --permissive
# --out-dockerfile "/home/user/GoWorkSpace/src/github.com/eclipse/che-operator/olm/test/Dockerfile"

# ${imageTool} build -t "${CATALOG_SOURCE_IMAGE}" -f index.Dockerfile .
# --skip-tls
# --permissive

popd || true
${imageTool} push "${CATALOG_SOURCE_IMAGE}" --cert-dir "${HOME}/crt"
#  --tls-verify=false

kubectl create secret docker-registry myregistrykey \
        --docker-server="${IMAGE_REGISTRY_HOST}" \
        --docker-username="kubeadmin" \
        --docker-password="$(oc whoami -t)" \
        --docker-email="test@example.com"
kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "myregistrykey"}]}'

yq -r "(.spec.template.spec.containers[0].image) = \"${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}\"" "${BASE_DIR}/olm/force-pulling-olm-images-job.yaml" | kubectl apply -f - -n "${namespace}"

kubectl wait --for=condition=complete --timeout=30s job/force-pulling-olm-images-job -n "${namespace}"

kubectl delete job/force-pulling-olm-images-job -n "${namespace}"

channel=nightly
packageName=eclipse-che-preview-${platform}
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  sourceType: grpc
  image: ${CATALOG_SOURCE_IMAGE}
  secrets:
    - 'myregistrykey'
  updateStrategy:
    registryPoll:
      interval: 5m
EOF

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: ${namespace}
spec:
  targetNamespaces:
  - ${namespace}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  channel: ${channel}
  installPlanApproval: Manual
  name: ${packageName}
  source: ${packageName}
  sourceNamespace: ${namespace}
EOF

#   startingCSV: ${CSV_NAME}

echo "[INFO] Subscribe to package"
kubectl describe subscription/"${packageName}" -n "${namespace}"

kubectl wait subscription/"${packageName}" -n "${namespace}" --for=condition=InstallPlanPending --timeout=240s
if [ $? -ne 0 ]; then
    echo Subscription failed to install the operator
    exit 1
fi

kubectl describe subscription/"${packageName}" -n "${namespace}"


echo "[INFO] Install operator package ${packageName} into namespace ${namespace}"
installPlan=$(kubectl get subscription/"${packageName}" -n "${namespace}" -o jsonpath='{.status.installplan.name}')

kubectl patch installplan/"${installPlan}" -n "${namespace}" --type=merge -p '{"spec":{"approved":true}}'

kubectl wait installplan/"${installPlan}" -n "${namespace}" --for=condition=Installed --timeout=240s
if [ $? -ne 0 ]; then
    echo InstallPlan failed to install the operator
    exit 1
fi

echo "Done"
