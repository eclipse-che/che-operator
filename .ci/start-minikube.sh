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

# Configure firewall rules for docker0 network
firewall-cmd --permanent --zone=trusted --add-interface=docker0
firewall-cmd --reload
firewall-cmd --get-active-zones
firewall-cmd --list-all --zone=trusted

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
minikube start --kubernetes-version=$KUBERNETES_VERSION --extra-config=apiserver.authorization-mode=RBAC



# waiting for node(s) to be ready
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'; until kubectl get nodes -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done

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

# Add minikube ingress
minikube addons enable ingress

echo "[INFO] Enable registry addon."
minikube addons enable registry

sleep 7

echo "Minikube Addon list"
minikube addons  list

# docker rm -f "$(docker ps -aq --filter "name=minikube-socat")" || true
# docker run --detach --rm --name="minikube-socat" --network=host alpine ash -c "apk add socat && socat TCP-LISTEN:5000,reuseaddr,fork TCP:$(minikube ip):5000"
# Todo drop socat container after the test...
echo "[INFO] List containers:==========="
docker ps -a
echo "=================================="

IP=$(minikube ip)
echo "[INFO] ================Minikube ip: ${IP}"

echo "[INFO] ================List services"
kubectl get service --all-namespaces
kubectl get service registry -n kube-system -o yaml

# Ping private image registry...
curl -X GET 0.0.0.0:5000/v2/_catalog || true
curl -X GET "${IP}:5000/v2/_catalog" || true

uname -a
echo "[INFO] List pods in the kube-system namespace"
kubectl get pod -n kube-system

echo "[INFO] Trying to get pod name of the registry proxy..."
REGISTRY_PROXY_POD=$(kubectl get pods -n kube-system -o yaml | grep  "name: registry-proxy-" | sed -e 's;.*name: \(\);\1;') || true
echo "[INFO] So proxy pod name is ${REGISTRY_PROXY_POD}"
echo "[INFO] Ok, let's take a look, what is going on inside registry proxy pod"
kubectl wait --for=condition=ready "pods/${REGISTRY_PROXY_POD}" --timeout=120s -n "kube-system" || true
kubectl logs "${REGISTRY_PROXY_POD}" -n kube-system || true
# echo "[INFO] Push test image...."
# docker pull alpine
# docker tag alpine "0.0.0.0:5000/alpine"
# docker push "0.0.0.0:5000/alpine"
# echo "[INFO] Test push done!"

echo "Minikube start is done!"
