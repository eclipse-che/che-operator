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
# download oc
echo "Download oc client"
mkdir -p ${OPERATOR_REPO}/tmp
chmod -R 777 ${OPERATOR_REPO}/tmp
wget https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz -O ${OPERATOR_REPO}/tmp/oc.tar && tar -xvf ${OPERATOR_REPO}/tmp/oc.tar -C ${OPERATOR_REPO}/tmp --strip-components=1

# start OKD
echo "Starting OKD 3.11"
cd ${OPERATOR_REPO}/tmp
rm -rf openshift.local.clusterup
./oc cluster up --public-hostname=172.17.0.1 --routing-suffix=172.17.0.1.nip.io
./oc login -u system:admin
./oc adm policy add-cluster-role-to-user cluster-admin developer
./oc login -u developer -p password
sleep 10
echo "Registering a custom resource definition"
./oc apply -f ${OPERATOR_REPO}/deploy/crds/org_v1_che_crd.yaml

# generate self signed cert
echo "Generating self signed certificate"
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -subj '/CN=*.172.17.0.1.nip.io' -nodes
cat cert.pem key.pem > ca.crt

# replace default router cert
echo "Updating OpenShift router tls secret"
./oc project default
./oc secrets new router-certs tls.crt=ca.crt tls.key=key.pem -o json --type='kubernetes.io/tls' --confirm | ./oc replace -f -
echo "Initiating a new router deployment"
sleep 10
./oc rollout latest dc/router -n=default

echo "Compiling tests binary"
docker run -t \
            -v ${OPERATOR_REPO}/tmp:/operator \
            -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.access.redhat.com/devtools/go-toolset-rhel7:1.11.5-3 \
            sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /operator/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"

cp ${OPERATOR_REPO}/tmp/run-tests ${OPERATOR_REPO}/run-tests


cd ${OPERATOR_REPO}
echo "Building operator docker image..."
docker build -t che/operator -f Dockerfile.ci .
echo "Running tests..."
./run-tests
