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

# CRC environments config

export CRC_VERSION=1.9.0
export SecretFile=pull-secrets.txt
export RAM_MEMORY=16384
export CPUS=4

set -e -x
curl -SLO  https://mirror.openshift.com/pub/openshift-v4/clients/crc/${CRC_VERSION}/crc-linux-amd64.tar.xz
tar -xvf crc-linux-amd64.tar.xz --strip-components=1
chmod +x ./crc
mv ./crc /usr/local/bin/crc

crc version
crc config set skip-check-root-user true
crc setup
crc start --cpus=${CPUS} --memory=${RAM_MEMORY} --pull-secret-file=${SecretFile} -n 8.8.8.8 --log-level debug
y | crc delete
