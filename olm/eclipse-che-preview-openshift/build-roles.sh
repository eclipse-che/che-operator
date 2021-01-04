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

BASE_DIR=$(cd "$(dirname "$0")" && pwd)
rm -Rf "${BASE_DIR}/generated/roles"
mkdir -p "${BASE_DIR}/generated/roles"
cp "${BASE_DIR}/../../deploy/role.yaml" "${BASE_DIR}/generated/roles/role.yaml"
cp "${BASE_DIR}/../../deploy/cluster_role.yaml" "${BASE_DIR}/generated/roles/cluster_role.yaml"
cp "${BASE_DIR}/../../deploy/namespaces_cluster_role.yaml" "${BASE_DIR}/generated/roles/namespaces_cluster_role.yaml"
