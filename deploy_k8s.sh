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
set -e
set -x

if [ -z "${1}" ]; then
  echo "missing namespace parameter './deploy_k8s.sh <che-namespace>'"
  exit 1
fi
NAMESPACE=${1}

BASE_DIR=$(cd "$(dirname "$0")"; pwd)

kubectl apply -f "${BASE_DIR}"/deploy/service_account.yaml -n="${NAMESPACE}"
kubectl apply -f "${BASE_DIR}"/deploy/role.yaml -n="${NAMESPACE}"
kubectl apply -f "${BASE_DIR}"/deploy/role_binding.yaml -n="${NAMESPACE}"

kubectl apply -f "${BASE_DIR}"/deploy/cluster_role.yaml
kubectl apply -f "${BASE_DIR}"/deploy/cluster_role_che.yaml
kubectl apply -f "${BASE_DIR}"/deploy/cluster_role_createns.yaml

kubectl apply -f "${BASE_DIR}"/deploy/cluster_role_binding.yaml -n="${NAMESPACE}"
kubectl apply -f "${BASE_DIR}"/deploy/cluster_role_binding_che.yaml -n="${NAMESPACE}"
kubectl apply -f "${BASE_DIR}"/deploy/cluster_role_binding_createns.yaml -n="${NAMESPACE}"

kubectl apply -f "${BASE_DIR}"/deploy/crds/org_v1_che_crd.yaml -n="${NAMESPACE}"

# sometimes the operator cannot get CRD right away
sleep 2

kubectl apply -f "${BASE_DIR}"/deploy/operator.yaml -n="${NAMESPACE}"
kubectl apply -f "${BASE_DIR}"/deploy/crds/org_v1_che_cr.yaml -n="${NAMESPACE}"
