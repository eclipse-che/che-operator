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

#docker build -t che/operator .
kubectl apply -f ${BASE_DIR}/deploy/service_account.yaml -n=$1
kubectl apply -f ${BASE_DIR}/deploy/role.yaml -n=$1
kubectl apply -f ${BASE_DIR}/deploy/role_binding.yaml -n=$1
kubectl apply -f ${BASE_DIR}/deploy/crds/org_v1_che_crd.yaml -n=$1
# sometimes the operator cannot get CRD right away
sleep 2
# uncomment when on OpenShift if you need login with OpenShift in Che
#oc new-app -f ${BASE_DIR}/deploy/role_binding_oauth.yaml -p NAMESPACE=$1 -n=$1
#oc apply -f ${BASE_DIR}/deploy/cluster_role.yaml -n=$1
kubectl apply -f ${BASE_DIR}/deploy/operator-local.yaml -n=$1
kubectl apply -f ${BASE_DIR}/deploy/crds/org_v1_che_cr.yaml -n=$1
