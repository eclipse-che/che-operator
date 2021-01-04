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
set -ex

# Minikube environments config
export MINIKUBE_VERSION=v1.8.2
export KUBERNETES_VERSION=v1.16.2
export MINIKUBE_HOME=$HOME
export CHANGE_MINIKUBE_NONE_USER=true
export KUBECONFIG=$HOME/.kube/config
export TEST_OUTPUT=1

sudo mount --make-rshared /
sudo mount --make-rshared /proc
sudo mount --make-rshared /sys

# Download minikube binary
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/$KUBERNETES_VERSION/bin/linux/amd64/kubectl && \
  chmod +x kubectl &&  \
sudo mv kubectl /usr/local/bin/

# Download minikube binary
curl -Lo minikube https://storage.googleapis.com/minikube/releases/$MINIKUBE_VERSION/minikube-linux-amd64 && \
  chmod +x minikube && \
  sudo mv minikube /usr/local/bin/

# Create kube folder
mkdir "${HOME}"/.kube || true
touch "${HOME}"/.kube/config

# minikube config
minikube config set WantUpdateNotification false
minikube config set WantReportErrorPrompt false
minikube config set WantNoneDriverWarning false
minikube config set vm-driver none
minikube version

# minikube start
sudo minikube start --kubernetes-version=$KUBERNETES_VERSION --extra-config=kubelet.resolv-conf=/run/systemd/resolve/resolv.conf
sudo chown -R $USER $HOME/.kube $HOME/.minikube

minikube update-context

#Give god access to the k8s API
kubectl apply -f - <<EOF
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: cluster-reader
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
  - nonResourceURLs: ["*"]
    verbs: ["*"]
EOF

echo "[INFO] Enable ingress addon."
sudo minikube addons enable ingress

echo "[INFO] Enable registry addon."
sudo minikube addons enable registry

echo "[INFO] Minikube Addon list"
sudo minikube addons  list

echo "[INFO] Trying to get pod name of the registry proxy..."
REGISTRY_PROXY_POD=$(sudo kubectl get pods -n kube-system -o yaml | grep  "name: registry-proxy-" | sed -e 's;.*name: \(\);\1;') || true
echo "[INFO] Proxy pod name is ${REGISTRY_PROXY_POD}"
sudo kubectl wait --for=condition=ready "pods/${REGISTRY_PROXY_POD}" --timeout=120s -n "kube-system" || true

echo "[INFO] Minikube started!"
