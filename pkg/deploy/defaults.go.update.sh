#!/bin/bash

# check for latest tags in upstream container repos

defaultsFile=${0%/*}/defaults.go

if [[ ! -f ${defaultsFile} ]]; then
    echo "$defaultsFile not found, downloading from master..."
    defaultsFile="/tmp/defaults.go"
    curl -ssL https://raw.githubusercontent.com/eclipse/che-operator/master/pkg/deploy/defaults.go -o ${defaultsFile}
fi

excludes="eclipse/che-keycloak|centos/postgresql-96-centos7"
for d in $(cat /tmp/defaults.go | egrep "Keycloak|Postgres|Pvc" | egrep Image | egrep -v "func|return|Old|ToDetect|$excludes" | sed -e "s#.\+= \"\(.\+\)\"#\1#"); do
    echo "- ${d}"
    echo -n "+ ${d%:*}:"; 
    e=$(skopeo inspect docker://${d%:*}  | yq .RepoTags | egrep -v "\[|\]|latest" | tr -d ",\" " | sort -V | tail -1)
    echo ${e}
    sed -i ${defaultsFile} -e "s@${d}@${d%:*}:${e}@g"
done

echo "Defaults updated in ${defaultsFile}. Don't forget to commit your changes!"
