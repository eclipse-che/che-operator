#!/bin/bash
set -e

if [ $# -ne 1 ]; then
    echo -e "Wrong number of parameters.\nUsage: ./loca-debug.sh <custom-resource-yaml>\n"
    exit 1
fi

command -v delv >/dev/null 2>&1 || { echo "operator-sdk is not installed. Aborting."; exit 1; }
command -v operator-sdk >/dev/null 2>&1 || { echo -e $RED"operator-sdk is not installed. Aborting."$NC; exit 1; }

CHE_NAMESPACE=che

kubectl create namespace $CHE_NAMESPACE
kubectl apply -f deploy/crds/org_v1_che_crd.yaml
kubectl apply -f $1 -n che

operator-sdk up local --namespace=${CHE_NAMESPACE} --enable-delve
