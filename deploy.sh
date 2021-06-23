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

BASE_DIR=$(cd "$(dirname "$0")"; pwd)

NAMESPACE=$1

oc apply -f ${BASE_DIR}/deploy/service_account.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/role.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/role_binding.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/cluster_role.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/cluster_role_binding.yaml -n $NAMESPACE

oc apply -f ${BASE_DIR}/deploy/crds/org_v1_che_crd.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd.yaml -n $NAMESPACE
# sometimes the operator cannot get CRD right away
sleep 2

oc apply -f ${BASE_DIR}/deploy/operator.yaml -n $NAMESPACE
oc apply -f ${BASE_DIR}/deploy/crds/org_v1_che_cr.yaml -n $NAMESPACE
