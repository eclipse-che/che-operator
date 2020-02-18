#!/bin/bash
#
# Copyright (c) 2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e -x

init() {
    SCRIPT=$(readlink -f "$0")
    SCRIPTPATH=$(dirname "$SCRIPT")
    CATALOG_IMAGENAME=olm_catalog

    if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
    else
    OPERATOR_REPO=$(dirname "$SCRIPTPATH");
    fi
}

create_csource_image() {
    install_required_packages
    installStartDocker
    docker build -t ${CATALOG_IMAGENAME} -f ${OPERATOR_REPO}/olm/eclipse-che-preview-${platform}/Dockerfile \
        ${OPERATOR_REPO}/olm/eclipse-che-preview-${platform}
    echo "[INFO] Successfully builded docker catalogSource image for ${platform} platform."
}

run_build() {
    for platform in 'kubernetes' 'openshift'
    do
        create_csource_image
    done
}

init
source ${OPERATOR_REPO}/.ci/util/ci_common.sh
run_build
