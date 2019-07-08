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

BASE_DIR=$(cd "$(dirname "$0")"; pwd)
rm -Rf ${BASE_DIR}/generated/roles
mkdir -p ${BASE_DIR}/generated/roles
yq -r '.channels[] | select(.name == "nightly") | .currentCSV' ${BASE_DIR}/../../deploy/role.yaml ${packageFilePath} > ${BASE_DIR}/generated/roles/role.yaml
