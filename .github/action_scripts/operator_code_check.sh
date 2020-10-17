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

set -e

# PR_FILES_CHANGED store all Modified/Created files in Pull Request.
export PR_FILES_CHANGED=$(git --no-pager diff --name-only HEAD "$(git merge-base HEAD origin/master)")
echo "========================="
echo "${PR_FILES_CHANGED}"
echo "========================="

# transform_files function transform PR_FILES_CHANGED into a new array => FILES_CHANGED_ARRAY.
function transform_files() {
    for files in ${PR_FILES_CHANGED} 
    do
        FILES_CHANGED_ARRAY+=("${files}")
    done
}

# check_che_types function check first if pkg/apis/org/v1/che_types.go file suffer modifications and
# in case of modification should exist also modifications in deploy/crds/* folder.
function check_che_types() {
    # CHE_TYPES_FILE make reference to generated code by operator-sdk.
    local CHE_TYPES_FILE='pkg/apis/org/v1/che_types.go'
    # Export variables for cr/crds files.
    local CR_CRD_FOLDER="deploy/crds/"
    local CR_CRD_REGEX="\S*org_v1_che_crd.yaml"

    if [[ " ${FILES_CHANGED_ARRAY[*]} " =~ ${CHE_TYPES_FILE} ]]; then
        echo "[INFO] File ${CHE_TYPES_FILE} suffer modifications in PR. Checking if exist modifications for cr/crd files."
        # The script should fail if deploy/crds folder didn't suffer any modification.
        if [[ " ${FILES_CHANGED_ARRAY[*]} " =~ $CR_CRD_REGEX ]]; then
            echo "[INFO] CR/CRD file modified: ${BASH_REMATCH}"
        else
            echo "[ERROR] Detected modification in ${CHE_TYPES_FILE} file, but cr/crd files didn't suffer any modification."
            exit 1
        fi
    else
        echo "[INFO] ${CHE_TYPES_FILE} don't have any modification."
    fi
}

transform_files
check_che_types

echo "[INFO] Done."
