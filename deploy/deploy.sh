#!/bin/bash

set -e
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
if [[ $1 = "k8s" ]]; then
    CMD="kubctls"
    CREATE_PROJECT="create namespace"
else
    CREATE_PROJECT="new-project"
    CMD="oc"
fi

if [[ -z "$2" ]]; then
    NAMESPACE="eclipse-che"
else
    NAMESPACE=$2
fi

${CMD} ${CREATE_PROJECT} ${NAMESPACE}
${CMD} create serviceaccount che-operator -n=${NAMESPACE}
${CMD} create rolebinding che-operator --clusterrole=admin --serviceaccount=${NAMESPACE}:che-operator -n=${NAMESPACE}

${CMD} create -f ${BASE_DIR}/config.yaml -n=${NAMESPACE}

${CMD} run -ti "che-operator" \
        --restart='Never' \
        --serviceaccount='che-operator' \
        --image='eivantsov/che-operator' \
        --env="OPERATOR_NAME=che-operator" \
        --overrides='{"spec":{"containers":[{"image": "eivantsov/che-operator", "name": "che-operator", "imagePullPolicy":"Never","envFrom":[{"configMapRef":{"name":"che-operator"}}]}]}}' \
        -n=${NAMESPACE}

