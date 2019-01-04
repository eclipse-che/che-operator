#!/bin/bash
#
# Copyright (c) 2012-2018 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#set -e
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
if [[ $1 = "k8s" || $1 = "minikube" ]]; then
    CMD="kubectl"
    CREATE_PROJECT="create namespace"
else
    CREATE_PROJECT="new-project"
    CMD="oc"
        OPENSHIFT_API_URL="$(oc whoami --show-server)"
fi

if [[ $1 = "minikube" ]]; then
    echo "Using MiniKube VM Docker daemon"
    eval $(minikube docker-env)
    MINIKUBE_IP="$(minikube ip)"
    if [[ -z "${MINIKUBE_IP}" ]]; then
        echo "Failed to get MiniKube IP. Make sure MiniKube is running. Current status:"
        minikube status
        exit 1
    fi
    INGRESS_DOMAIN="${MINIKUBE_IP}.nip.io"
    echo "Using MiniKube ingress domain: "${INGRESS_DOMAIN}
 fi

echo "Building Che Operator..."
docker build -t che-operator ${BASE_DIR}/../
if [[ $? -ne 0 ]]; then
    echo "Failed to build an Operator"
    exit 1
fi

if [[ -z "$2" ]]; then
    NAMESPACE="eclipse-che"
else
    NAMESPACE=$2
fi

${CMD} ${CREATE_PROJECT} ${NAMESPACE}

if [[ $? -ne 0 ]]; then
    echo "Namespace ${NAMESPACE} cannot be crated. Generating namespace name"
    NAMESPACE="eclipse-che$(( ( RANDOM % 10 )  + 1 ))$(( ( RANDOM % 10 )  + 1 ))"
    ${CMD} ${CREATE_PROJECT} ${NAMESPACE}
fi

${CMD} create serviceaccount che-operator -n=${NAMESPACE}
${CMD} create rolebinding che-operator --clusterrole=admin --serviceaccount=${NAMESPACE}:che-operator -n=${NAMESPACE}
${CMD} create -f ${BASE_DIR}/config.yaml -n=${NAMESPACE}
${CMD} patch cm/che-operator -p "{\"data\": {\"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN\": \"${INGRESS_DOMAIN}\", \"CHE_OPENSHIFT_API_URL\": \"${OPENSHIFT_API_URL}\"}}" -n ${NAMESPACE}
${CMD} delete pod che-operator -n=${NAMESPACE}  2> /dev/null || true
${CMD} run -ti "che-operator" \
        --restart='Never' \
        --serviceaccount='che-operator' \
        --image='che-operator' \
        --env="OPERATOR_NAME=che-operator" \
        --overrides='{"spec":{"containers":[{"image": "che-operator", "name": "che-operator", "imagePullPolicy":"IfNotPresent","envFrom":[{"configMapRef":{"name":"che-operator"}}]}]}}' \
        -n=${NAMESPACE}
