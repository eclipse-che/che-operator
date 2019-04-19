#!/bin/bash -e
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

# to purge ALL existing docker containers (including unrelated ones!)
# docker rm -f $(docker ps -aq) || true
# # to purge ALL existing docker images (including unrelated ones!)
# docker rmi -f $(docker images -q) || true

# requires Docker 18+

OC_VERSION="v3.11.0-0cbc58b"
GO_TOOLSET_VERSION="1.11.5-3"
IP_ADDRESS="172.17.0.1"

SCRIPT=$(readlink -f "$0") # this script's absolute path
SCRIPTPATH=$(dirname "$SCRIPT") # /path/to/e2e/ folder
if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then OPERATOR_REPO=${WORKSPACE}; else OPERATOR_REPO=$(dirname "$SCRIPTPATH"); fi

# download oc client binary
echo "[INFO] Download oc client"
mkdir -p ${OPERATOR_REPO}/tmp
chmod -R 777 ${OPERATOR_REPO}/tmp
curl -s -S -L https://github.com/openshift/origin/releases/download/${OC_VERSION%%-*}/openshift-origin-client-tools-${OC_VERSION}-linux-64bit.tar.gz \
  -o ${OPERATOR_REPO}/tmp/oc.tar && tar -xvf ${OPERATOR_REPO}/tmp/oc.tar -C ${OPERATOR_REPO}/tmp --strip-components=1

# start OKD
echo "[INFO] Start OKD ${OC_VERSION}"
cd ${OPERATOR_REPO}/tmp
rm -rf openshift.local.clusterup
./oc cluster up --public-hostname=${IP_ADDRESS} --routing-suffix=${IP_ADDRESS}.nip.io
./oc login -u system:admin
./oc adm policy add-cluster-role-to-user cluster-admin developer
./oc login -u developer -p password
sleep 10
echo "[INFO] Register a custom resource definition"
./oc apply -f ${OPERATOR_REPO}/deploy/crds/org_v1_che_crd.yaml

# generate self signed cert
echo "[INFO] Generate self signed certificate"
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -subj "/CN=*.${IP_ADDRESS}.nip.io" -nodes
cat cert.pem key.pem > ca.crt

# replace default router cert
echo "[INFO] Update OpenShift router tls secret"
./oc project default
./oc secrets new router-certs tls.crt=ca.crt tls.key=key.pem -o json --type='kubernetes.io/tls' --confirm | ./oc replace -f -
echo "[INFO] Initiate a new router deployment"
sleep 20
./oc rollout latest dc/router -n=default || true

echo "[INFO] Compile tests binary"
docker run -t \
            -v ${OPERATOR_REPO}/tmp:/operator \
            -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.access.redhat.com/devtools/go-toolset-rhel7:${GO_TOOLSET_VERSION} \
            sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /operator/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"

cp ${OPERATOR_REPO}/tmp/run-tests ${OPERATOR_REPO}/run-tests

echo "[INFO] Build operator docker image..."
cd ${OPERATOR_REPO} && docker build -t che/operator -f Dockerfile .

echo "[INFO] Run tests..."
cd ${OPERATOR_REPO} && ./run-tests
