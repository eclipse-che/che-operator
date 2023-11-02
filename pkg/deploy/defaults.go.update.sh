#!/bin/bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# check for latest tags in upstream container repos

defaultsFile=${0%/*}/defaults.go

if [[ ! -f ${defaultsFile} ]]; then
    echo "$defaultsFile not found, downloading from main..."
    defaultsFile="/tmp/defaults.go"
    curl -ssL https://raw.githubusercontent.com/eclipse-che/che-operator/main/pkg/deploy/defaults.go -o ${defaultsFile}
fi

excludes="eclipse|centos/postgresql-96-centos7"
for d in $(cat /tmp/defaults.go | egrep "Postgres|Pvc" | egrep Image | egrep -v "func|return|Old|ToDetect|$excludes" | sed -e "s#.\+= \"\(.\+\)\"#\1#"); do
    echo "- ${d}"
    echo -n "+ ${d%:*}:";
    e=$(skopeo inspect docker://${d%:*}  | yq .RepoTags | egrep -v "\[|\]|latest" | tr -d ",\" " | sort -V | tail -1)
    echo ${e}
    sed -i ${defaultsFile} -e "s@${d}@${d%:*}:${e}@g"
done

echo "Defaults updated in ${defaultsFile}. Don't forget to commit your changes!"
