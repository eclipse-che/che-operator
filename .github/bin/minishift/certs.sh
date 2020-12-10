#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

# Please note. This certificates are generated only in MacOS
export CA_CN="Local Eclipse Che Signer"
export DOMAIN=\*.$( minishift ip ).nip.io
export OPENSSL_CNF=/System/Library/OpenSSL/openssl.cnf

openssl genrsa -out ca.key 4096
openssl req -x509 \
    -new -nodes \
    -key ca.key \
    -sha256 \
    -days 1024 \
    -out ca.crt \
    -subj /CN="${CA_CN}" \
    -reqexts SAN \
    -extensions SAN \
    -config <(cat ${OPENSSL_CNF} \
    <(printf '[SAN]\nbasicConstraints=critical, CA:TRUE\nkeyUsage=keyCertSign, cRLSign, digitalSignature'))
    openssl genrsa -out domain.key 2048

openssl req -new -sha256 \
    -key domain.key \
    -subj "/O=Local Eclipse Che/CN=${DOMAIN}" \
    -reqexts SAN \
    -config <(cat ${OPENSSL_CNF} \
    <(printf "\n[SAN]\nsubjectAltName=DNS:${DOMAIN}\nbasicConstraints=critical, CA:FALSE\nkeyUsage=digitalSignature, keyEncipherment, keyAgreement, dataEncipherment\nextendedKeyUsage=serverAuth")) \
    -out domain.csr

    openssl x509 \
    -req \
    -sha256 \
    -extfile <(printf "subjectAltName=DNS:${DOMAIN}\nbasicConstraints=critical, CA:FALSE\nkeyUsage=digitalSignature, keyEncipherment, keyAgreement, dataEncipherment\nextendedKeyUsage=serverAuth") \
    -days 365 \
    -in domain.csr \
    -CA ca.crt \
    -CAkey ca.key \
    -CAcreateserial -out domain.crt

# Add the newer minishift certificate to minishift router-certs
sleep 60
eval $(minishift oc-env)

oc login -u system:admin --insecure-skip-tls-verify=true
oc create namespace eclipse-che
oc project default

oc delete secret router-certs
cat domain.crt domain.key > minishift.crt
oc create secret tls router-certs --key=domain.key --cert=minishift.crt
oc rollout latest router
oc create secret generic self-signed-certificate --from-file=ca.crt -n=eclipse-che
oc adm policy add-cluster-role-to-user cluster-admin developer && oc login -u developer -p developer 
