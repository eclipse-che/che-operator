#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

# Todo remove this code. I wrote it like a way to get the latest che-operator version...
# But looks like we can do this staff using github api: retrieve list tags and sort them by creation time.

versions=($(curl --silent "https://api.github.com/repos/eclipse/che-operator/tags" | yq -r " .[].name | sub(\"v\"; \"\") " ))
echo "${versions[*]}"

sortedVersions=()

findMaxElem() {
    arr=("${@}")
    MAX="${arr[0]}"
    MAX_INDEX=0

    for index in "${!arr[@]}"; do
        compareResult=$(pysemver compare "${arr[index]}" "${MAX}")
        if [ "${compareResult}" == "1" ] || [ "${compareResult}" == "0" ]; then
            MAX="${arr[index]}"
            MAX_INDEX=${index}
        fi
    done

    sortedVersions+=("${MAX}")
    # Remove element from array
    printf "="
    unset "arr[${MAX_INDEX}]"
}

function sort() {
    versions=("${@}")
    findMaxElem "${versions[@]}"
    if [ ! ${#arr[@]} -eq 0 ]; then
        sort "${arr[@]}"
    else
        printf ">Version sorting completed."
    fi
}

installSemverPython() {
  PySemver=$(command -v pysemver) || true
  if [[ ! -x "${PySemver}" ]]; then
    pip3 install semver
  fi
  echo "[INFO] $(pysemver --version)"
}

installSemverPython
sort "${versions[@]}"

echo "sorted versions: ${sortedVersions[*]}"
