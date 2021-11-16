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

# Scripts to prepare OLM(operator lifecycle manager) and install che-operator package
# with specific version using OLM.

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
