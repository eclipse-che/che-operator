#!/usr/bin/env bash
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

set -e
set -x

# Get absolute path for root repo directory from github actions context: https://docs.github.com/en/free-pro-team@latest/actions/reference/context-and-expression-syntax-for-github-actions
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
if [ -z "${OPERATOR_REPO}" ]; then
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
fi

source "${OPERATOR_REPO}/build/scripts/minikube-tests/common.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  buildAndCopyCheOperatorImageToMinikube
  yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml"
  yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator/kubernetes/operator.yaml"

  chectl server:deploy \
    --batch \
    --platform minikube \
    --k8spodwaittimeout=6000000 \
    --k8spodreadytimeout=6000000 \
    --templates "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}" \
    --che-operator-cr-patch-yaml "${OPERATOR_REPO}/build/scripts/minikube-tests/minikube-checluster-patch.yaml"

  make wait-devworkspace-running NAMESPACE="devworkspace-controller" VERBOSE=1

  # Free up some cpu resources
  kubectl scale deployment che --replicas=0 -n eclipse-che

  createDevWorkspaceTest
  startAndWaitDevWorkspaceTest
  stopAndWaitDevWorkspace
  deleteDevWorkspace
}

createDevWorkspaceTest() {
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${USER_NAMESPACE}
  annotations:
    che.eclipse.org/username: admin
  labels:
    app.kubernetes.io/component: workspaces-namespace
    app.kubernetes.io/part-of: che.eclipse.org
    kubernetes.io/metadata.name: ${USER_NAMESPACE}
EOF

kubectl apply -f - <<EOF
apiVersion: workspace.devfile.io/v1alpha2
kind: DevWorkspaceTemplate
metadata:
  name: che-code
  namespace: ${USER_NAMESPACE}
spec:
  commands:
  - apply:
      component: che-code-injector
    id: init-container-command
  - exec:
      commandLine: nohup /checode/entrypoint-volume.sh > /checode/entrypoint-logs.txt
        2>&1 &
      component: che-code-runtime-description
    id: init-che-code-command
  components:
  - container:
      command:
      - /entrypoint-init-container.sh
      cpuLimit: 500m
      cpuRequest: 30m
      env:
      - name: CHE_DASHBOARD_URL
        value: https://$(minikube ip).nip.io
      - name: OPENVSX_REGISTRY_URL
        value: https://open-vsx.org
      image: quay.io/abazko/che-code:next
      memoryLimit: 256Mi
      memoryRequest: 32Mi
      sourceMapping: /projects
      volumeMounts:
      - name: checode
        path: /checode
    name: che-code-injector
  - attributes:
      app.kubernetes.io/component: che-code-runtime
      app.kubernetes.io/part-of: che-code.eclipse.org
      controller.devfile.io/container-contribution: true
    container:
      cpuLimit: 500m
      cpuRequest: 30m
      endpoints:
      - attributes:
          cookiesAuthEnabled: true
          discoverable: false
          type: main
          urlRewriteSupported: true
        exposure: public
        name: che-code
        protocol: https
        secure: true
        targetPort: 3100
      - attributes:
          discoverable: false
          urlRewriteSupported: false
        exposure: public
        name: code-redirect-1
        protocol: https
        targetPort: 13131
      - attributes:
          discoverable: false
          urlRewriteSupported: false
        exposure: public
        name: code-redirect-2
        protocol: https
        targetPort: 13132
      - attributes:
          discoverable: false
          urlRewriteSupported: false
        exposure: public
        name: code-redirect-3
        protocol: https
        targetPort: 13133
      env:
      - name: CHE_DASHBOARD_URL
        value: https://$(minikube ip).nip.io
      - name: OPENVSX_REGISTRY_URL
        value: https://open-vsx.org
      image: quay.io/abazko/universal-developer-image:pr-3
      memoryLimit: 1024Mi
      memoryRequest: 256Mi
      sourceMapping: /projects
      volumeMounts:
      - name: checode
        path: /checode
    name: che-code-runtime-description
  - name: checode
    volume: {}
  events:
    postStart:
    - init-che-code-command
    preStart:
    - init-container-command
EOF

  kubectl apply -f - <<EOF
kind: DevWorkspace
apiVersion: workspace.devfile.io/v1alpha2
metadata:
  name: ${DEV_WORKSPACE_NAME}
  namespace: ${USER_NAMESPACE}
spec:
  contributions:
  - kubernetes:
      name: che-code
    name: editor
  routingClass: che
  started: false
  template:
    attributes:
      controller.devfile.io/storage-type: ephemeral
    components:
      - name: tooling-container
        container:
          image: quay.io/abazko/universal-developer-image:pr-3
          cpuLimit: 100m
          cpuRequest: 100m
EOF
}

startAndWaitDevWorkspaceTest() {
  # pre-pull image for faster workspace startup
  minikube image pull quay.io/abazko/universal-developer-image:pr-3
  minikube image pull quay.io/abazko/che-code:next

  kubectl patch devworkspace ${DEV_WORKSPACE_NAME} -p '{"spec":{"started":true}}' --type=merge -n ${USER_NAMESPACE}
  kubectl wait devworkspace ${DEV_WORKSPACE_NAME} -n ${USER_NAMESPACE} --for=jsonpath='{.status.phase}'=Running --timeout=300s
}


pushd ${OPERATOR_REPO} >/dev/null
initDefaults
initTemplates
runTest
popd >/dev/null


