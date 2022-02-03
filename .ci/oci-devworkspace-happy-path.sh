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

set -e
set -x

export CHE_REPO_BRANCH="main"

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
source "${OPERATOR_REPO}"/.github/bin/common.sh
source <(curl -s https://raw.githubusercontent.com/eclipse/che/${CHE_REPO_BRANCH}/tests/devworkspace-happy-path/common.sh)

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

overrideDefaults() {
  # CI_CHE_OPERATOR_IMAGE it is che operator image builded in openshift CI job workflow. More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  export OPERATOR_IMAGE=${CI_CHE_OPERATOR_IMAGE}
}

deployChe() {
  deployEclipseCheOnWithOperator "chectl" "openshift" ${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH} "true"
}

initDefaults
initTemplates
overrideDefaults
deployChe

bash <(curl -s https://raw.githubusercontent.com/eclipse/che/${CHE_REPO_BRANCH}/tests/devworkspace-happy-path/remote-launch.sh)
