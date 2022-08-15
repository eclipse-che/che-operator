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

# ---------------------------------------------------------------#
# This scripts check if all development resources are up to date #
#----------------------------------------------------------------#

set -e

OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")

pushd "${OPERATOR_REPO}"

# Update resources
make update-dev-resources INCREMENT_BUNDLE_VERSION=false

if [[ $(git diff --name-only | wc -l) != 0 ]]; then
  # Print difference
  git --no-pager diff

  echo "[ERROR] Resources are not up to date."
  echo "[ERROR] Run 'make update-dev-resources' to update them."
  exit 1
else
  echo "[INFO] Done."
fi

popd

