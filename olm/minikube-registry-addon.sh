#!/bin/bash

set -e

if [ -z "${BASE_DIR}" ]; then
  SCRIPT=$(readlink -f "$0")
  export SCRIPT

  BASE_DIR=$(dirname "$(dirname "$SCRIPT")")/olm;
  export BASE_DIR
fi

ROOT_DIR=$(dirname "${BASE_DIR}")

source ${ROOT_DIR}/olm/check-yq.sh

minikube addons enable registry
registryPod=$(kubectl get pods -n kube-system -o yaml | yq -r ".items[] | select(.metadata.labels.\"actual-registry\") | .metadata.name")
kubectl wait --for=condition=ready "pods/${registryPod}" --timeout=120s -n "kube-system"
kubectl port-forward --namespace kube-system "pod/${registryPod}" 5000:5000
