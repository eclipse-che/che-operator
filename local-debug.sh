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

command -v delv >/dev/null 2>&1 || { echo "operator-sdk is not installed. Aborting."; exit 1; }
command -v operator-sdk >/dev/null 2>&1 || { echo -e $RED"operator-sdk is not installed. Aborting."$NC; exit 1; }

CHE_NAMESPACE=che



set +e
kubectl create namespace $CHE_NAMESPACE
set -e

CR="${1}"
if [ -z "${CR}" ]; then
    CR="./deploy/crds/org_v1_che_cr.yaml"
    echo "[INFO] First argument is an empty. Set up default CR file: ${CR}"
fi

kubectl apply -f deploy/crds/org_v1_che_crd.yaml
kubectl apply -f "${CR}" -n $CHE_NAMESPACE
cp templates/keycloak_provision /tmp/keycloak_provision
cp templates/oauth_provision /tmp/oauth_provision

ENV_FILE=/tmp/che-operator-debug.env
rm -rf "${ENV_FILE}"
touch "${ENV_FILE}"
CLUSTER_API_URL=$(oc whoami --show-server=true) || true
if [ -n "${CLUSTER_API_URL}" ]; then 
    echo "CLUSTER_API_URL='${CLUSTER_API_URL}'" > "${ENV_FILE}"
    echo "[INFO] Set up cluster api url: ${CLUSTER_API_URL}"
fi

echo "[WARN] Make sure that your CR contains valid ingress domain!"

operator-sdk up local --namespace=${CHE_NAMESPACE} --enable-delve
