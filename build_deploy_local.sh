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

if [ -z "${1}" ]; then
  echo "missing namespace parameter './build_deploy_local.sh <che-namespace>'"
  exit 1
fi
NAMESPACE=${1}

docker build -t che/operator .
kubectl apply -f ${BASE_DIR}/deploy/service_account.yaml -n="${NAMESPACE}"
kubectl apply -f ${BASE_DIR}/deploy/role.yaml -n="${NAMESPACE}"
kubectl apply -f ${BASE_DIR}/deploy/role_binding.yaml -n="${NAMESPACE}"
kubectl apply -f ${BASE_DIR}/deploy/namespaces_cluster_role.yaml
kubectl apply -f ${BASE_DIR}/deploy/namespaces_cluster_role_binding.yaml
kubectl apply -f ${BASE_DIR}/deploy/crds/org_v1_che_crd.yaml -n="${NAMESPACE}"
# sometimes the operator cannot get CRD right away
sleep 2
# uncomment when on OpenShift if you need login with OpenShift in Che
#oc new-app -f ${BASE_DIR}/deploy/role_binding_oauth.yaml -p NAMESPACE="${NAMESPACE}" -n="${NAMESPACE}"
#oc apply -f ${BASE_DIR}/deploy/cluster_role.yaml -n="${NAMESPACE}"
kubectl apply -f ${BASE_DIR}/deploy/operator-local.yaml -n="${NAMESPACE}"
kubectl apply -f ${BASE_DIR}/deploy/crds/org_v1_che_cr.yaml -n="${NAMESPACE}"
