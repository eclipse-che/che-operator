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

oc apply -f ${BASE_DIR}/service_account.yaml
oc apply -f ${BASE_DIR}/role.yaml
oc apply -f ${BASE_DIR}/role_binding.yaml
oc apply -f ${BASE_DIR}/crds/org_v1_che_crd.yaml
# sometimes the operator cannot get CRD right away
sleep 2
# uncomment if you need Login with OpenShift and/or use self signed certificates and tls
#oc adm policy add-cluster-role-to-user cluster-admin -z che-operator
oc apply -f ${BASE_DIR}/operator.yaml
oc apply -f ${BASE_DIR}/crds/org_v1_che_cr.yaml