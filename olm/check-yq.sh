#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

if ! command -v yq >/dev/null 2>&1
then
  echo
  echo "#### ERROR ####"
  echo "####"
  echo "#### Please install the 'yq' tool before being able to use this script"
  echo "#### see https://github.com/kislyuk/yq"
  echo "#### and https://stedolan.github.io/jq/download"
  echo "####"
  echo "###############"
  exit 1
fi
