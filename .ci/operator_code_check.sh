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

# Perform git installation before execute.
yum -y install git

# PR_FILES_CHANGED store all Modified/Created files in Pull Request.
export PR_FILES_CHANGED=$(git --no-pager diff --name-only HEAD $(git merge-base HEAD origin/master))

# transform_files function transform PR_FILES_CHANGED into a new array => FILES_CHANGED_ARRAY.
function transform_files() {
    for files in ${PR_FILES_CHANGED} 
    do 
        FILES_CHANGED_ARRAY+=($files)
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

    if [[ " ${FILES_CHANGED_ARRAY[@]} " =~ " ${CHE_TYPES_FILE} " ]]; then
        echo "[INFO] File ${CHE_TYPES_FILE} suffer modifications in PR. Checking if exist modifications for cr/crd files."
        # The script should fail if deploy/crds folder didn't suffer any modification.
        if [[ " ${FILES_CHANGED_ARRAY[@]} " =~ $CR_CRD_REGEX ]]; then
            echo "[INFO] CR/CRD file modified: ${BASH_REMATCH}"
        else
            echo "[ERROR] Detected modification in ${CHE_TYPES_FILE} file, but cr/crd files didn't suffer any modification."
            exit 1
        fi
    else
        echo "[INFO] ${CHE_TYPES_FILE} don't have any modification."
    fi
}

# check_nightly_files checks if exist nightly files after checking if exist any changes in deploy folder
function check_nightly_files() {
    # Define olm-catalog folder and regexp to check if exist nightly files for kubernetes
    local OLM_KUBERNETES='olm/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes/'
    local OLM_K8S="\b$OLM_KUBERNETES.*?\b"

    # Define olm-catalog folder and regexp to check if exist nightly files for openshift
    local OLM_OPENSHIFT='olm/eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift/'
    local OLM_OCP="\b$OLM_OPENSHIFT.*?\b"

    # Match if exist nightly files in PR
    if [[ " ${FILES_CHANGED_ARRAY[@]} " =~ $OLM_K8S && " ${FILES_CHANGED_ARRAY[@]} " =~ $OLM_OCP ]]; then
        echo "[INFO] Nightly files for kubernetes and openshift platform was created."
        exit 0
    else
        echo "[ERROR] Nightly files for kubernetes and openshift platform not created."
        exit 1
    fi
}

#check_deploy_folder check first if files under deploy/* folder have modifications and in case of modification
# check if exist nightly files for kubernetes and openshift platform.
function check_deploy_folder() {
    # Define deploy folder and regexp to search all under deploy/*
    local CR_CRD_FOLDER="deploy/"

    # Checking if exist modifications in deploy folder
    for files in ${FILES_CHANGED_ARRAY[@]}
    do
        if [[ $files =~ ^$CR_CRD_FOLDER.*? ]]; then
            echo "[INFO] Deploy Folder suffer modifications. Checking if exist nightly files..."
            check_nightly_files
        fi
    done

    echo "[INFO] ${CR_CRD_FOLDER} don't have any modification."
}

transform_files
check_che_types
check_deploy_folder
