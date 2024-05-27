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

catchFinish() {
  result=$?

  collectLogs
  if [ "$result" != "0" ]; then
    echo "[ERROR] Job failed."
  else
    echo "[INFO] Job completed successfully."
  fi

  rm -rf ${OPERATOR_REPO}/tmp

  echo "[INFO] Please check github actions artifacts."
  exit $result
}

initDefaults() {
  export NAMESPACE="eclipse-che"
  export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/artifacts-che"}
  export CHECTL_TEMPLATES_BASE_DIR=/tmp/chectl-templates
  export OPERATOR_IMAGE="test/che-operator:test"
  export DEV_WORKSPACE_NAME="test-dev-workspace"
  export USER_NAMESPACE="admin-che"

  # turn off telemetry
  mkdir -p ${HOME}/.config/chectl
  echo "{\"segment.telemetry\":\"off\"}" > ${HOME}/.config/chectl/config.json

  getLatestStableVersions
}

initTemplates() {
  rm -rf ${OPERATOR_REPO}/tmp
  rm -rf ${CHECTL_TEMPLATES_BASE_DIR} && mkdir -p ${CHECTL_TEMPLATES_BASE_DIR} && chmod 777 ${CHECTL_TEMPLATES_BASE_DIR}

  PREVIOUS_OPERATOR_VERSION_CLONE_PATH=${OPERATOR_REPO}/tmp/${PREVIOUS_PACKAGE_VERSION}
  git clone --quiet --depth 1 --branch ${PREVIOUS_PACKAGE_VERSION} https://github.com/eclipse-che/che-operator/ ${PREVIOUS_OPERATOR_VERSION_CLONE_PATH}

  LAST_OPERATOR_VERSION_CLONE_PATH=${OPERATOR_REPO}/tmp/${LAST_PACKAGE_VERSION}
  git clone --quiet --depth 1 --branch ${LAST_PACKAGE_VERSION} https://github.com/eclipse-che/che-operator/ ${LAST_OPERATOR_VERSION_CLONE_PATH}

  # Find out DWO latest stable version
  git clone --quiet https://github.com/devfile/devworkspace-operator ${OPERATOR_REPO}/tmp/dwo
  pushd ${OPERATOR_REPO}/tmp/dwo
  DWO_STABLE_VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))
  popd

  export CURRENT_OPERATOR_VERSION_TEMPLATE_PATH=${CHECTL_TEMPLATES_BASE_DIR}
  export PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH=${CHECTL_TEMPLATES_BASE_DIR}/${PREVIOUS_PACKAGE_VERSION}
  export LAST_OPERATOR_VERSION_TEMPLATE_PATH=${CHECTL_TEMPLATES_BASE_DIR}/${LAST_PACKAGE_VERSION}

  pushd "${OPERATOR_REPO}" || exit 1
  make gen-chectl-tmpl SOURCE_PROJECT="${PREVIOUS_OPERATOR_VERSION_CLONE_PATH}" TEMPLATES="${PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH}" DWO_VERSION="${DWO_STABLE_VERSION}"
  make gen-chectl-tmpl SOURCE_PROJECT="${LAST_OPERATOR_VERSION_CLONE_PATH}" TEMPLATES="${LAST_OPERATOR_VERSION_TEMPLATE_PATH}" DWO_VERSION="${DWO_STABLE_VERSION}"
  make gen-chectl-tmpl SOURCE_PROJECT="${OPERATOR_REPO}" TEMPLATES="${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}" DWO_VERSION="main"
  popd || exit 1
}

getLatestStableVersions() {
  git remote add operator https://github.com/eclipse-che/che-operator.git
  git fetch operator -q
  tags=$(git ls-remote --refs --tags operator | sed -n 's|.*refs/tags/\(7.*\)|\1|p' | awk -F. '{ print ($1*1000)+($2*10)+$3" "$1"."$2"."$3}' | sort | tac)
  export PREVIOUS_PACKAGE_VERSION=$(echo "${tags}" | sed -n 2p | cut -d ' ' -f2)
  export LAST_PACKAGE_VERSION=$(echo "${tags}" | sed -n 1p | cut -d ' ' -f2)
  git remote remove operator
}

collectLogs() {
  mkdir -p ${ARTIFACTS_DIR}

  set +ex
  # Collect logs only for k8s cluster since OpenShift CI already dump all resources
  collectClusterResources

  # additionally grab server logs for fast access
  chectl server:logs -n $NAMESPACE -d $ARTIFACTS_DIR
  set -e
}

RESOURCES_DIR_NAME='resources'
NAMESPACED_DIR_NAME='namespaced'
CLUSTER_DIR_NAME='cluster'

collectClusterResources() {
  allNamespaces=$(kubectl get namespaces -o custom-columns=":metadata.name")
  for namespace in $allNamespaces ; do
    collectNamespacedScopeResources $namespace
    collectNamespacedPodLogs $namespace
    collectNamespacedEvents $namespace
  done
  collectClusterScopeResources
}

collectNamespacedScopeResources() {
  namespace="$1"
  if [[ -z $namespace ]]; then return; fi

  STANDARD_KINDS=(
                  "pods"
                  "jobs"
                  "deployments"
                  "services"
                  "ingresses"
                  "configmaps"
                  "secrets"
                  "serviceaccounts"
                  "roles"
                  "rolebindings"
                  "pvc"
                  )
  CRDS_KINDS=($(kubectl get crds -o jsonpath="{.items[*].spec.names.plural}"))
  KINDS=("${STANDARD_KINDS[@]}" "${CRDS_KINDS[@]}")

  for kind in "${KINDS[@]}" ; do
    dir="${ARTIFACTS_DIR}/${RESOURCES_DIR_NAME}/${NAMESPACED_DIR_NAME}/${namespace}/${kind}"
    mkdir -p $dir

    names=$(kubectl get -n $namespace $kind --no-headers=true -o custom-columns=":metadata.name")
    for name in $names ; do
      filename=${name//[:<>|*?]/_}
      kubectl get -n $namespace $kind $name -o yaml > "${dir}/${filename}.yaml"
    done
  done
}

collectNamespacedPodLogs() {
  namespace="$1"
  if [[ -z $namespace ]]; then return; fi

  dir="${ARTIFACTS_DIR}/${RESOURCES_DIR_NAME}/${NAMESPACED_DIR_NAME}/${namespace}/logs"
  mkdir -p $dir

  pods=$(kubectl get -n $namespace pods --no-headers=true -o custom-columns=":metadata.name")
  for pod in $pods ; do
    containers=$(kubectl get -n $namespace pod $pod -o jsonpath="{.spec.containers[*].name}")
    for container in $containers ; do
      filename=${pod//[:<>|*?]/_}_${container//[:<>|*?]/_}
      kubectl logs -n $namespace $pod -c $container > "${dir}/${filename}.log"
    done
  done
}

collectNamespacedEvents() {
  namespace="$1"
  if [[ -z $namespace ]]; then return; fi

  dir="${ARTIFACTS_DIR}/${RESOURCES_DIR_NAME}/${NAMESPACED_DIR_NAME}/${namespace}"
  mkdir -p $dir

  kubectl get -n $namespace events > "${dir}/events.yaml"
}

collectClusterScopeResources() {
  KINDS=(
        "crds"
        "pv"
        "clusterroles"
        "clusterrolebindings"
        )
  for kind in "${KINDS[@]}" ; do
    dir="${ARTIFACTS_DIR}/${RESOURCES_DIR_NAME}/${CLUSTER_DIR_NAME}/${kind}"
    mkdir -p $dir

    names=$(kubectl get -n $namespace $kind --no-headers=true -o custom-columns=":metadata.name")
    for name in $names ; do
      filename=${name//[:<>|*?]/_}
      kubectl get -n $namespace $kind $name -o yaml > "${dir}/${filename}.yaml"
    done
  done
}

buildAndCopyCheOperatorImageToMinikube() {
  docker build -t "${OPERATOR_IMAGE}" -f Dockerfile --build-arg SKIP_TESTS=true .
  docker save "${OPERATOR_IMAGE}" > /tmp/operator.tar
  eval $(minikube docker-env) && docker load -i  /tmp/operator.tar && rm  /tmp/operator.tar
}

createDevWorkspace() {
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
      image: quay.io/che-incubator/che-code:latest
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
      image: quay.io/devfile/universal-developer-image:ubi8-latest
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
  contributions:
    - name: ide
      uri: http://plugin-registry.eclipse-che.svc:8080/v3/plugins/che-incubator/che-code/insiders/devfile.yaml
  template:
    attributes:
      controller.devfile.io/storage-type: ephemeral
    components:
      - name: tooling-container
        container:
          image: quay.io/devfile/universal-developer-image:ubi8-latest
          cpuLimit: 100m
          cpuRequest: 100m
EOF
}

startAndWaitDevWorkspace() {
  # pre-pull image for faster workspace startup
  minikube image pull quay.io/devfile/universal-developer-image:ubi8-latest
  minikube image pull quay.io/che-incubator/che-code:latest

  kubectl patch devworkspace ${DEV_WORKSPACE_NAME} -p '{"spec":{"started":true}}' --type=merge -n ${USER_NAMESPACE}
  kubectl wait devworkspace ${DEV_WORKSPACE_NAME} -n ${USER_NAMESPACE} --for=jsonpath='{.status.phase}'=Running --timeout=300s
}

stopAndWaitDevWorkspace() {
  kubectl patch devworkspace ${DEV_WORKSPACE_NAME} -p '{"spec":{"started":false}}' --type=merge -n ${USER_NAMESPACE}
  kubectl wait devworkspace ${DEV_WORKSPACE_NAME} -n ${USER_NAMESPACE} --for=jsonpath='{.status.phase}'=Stopped
}

deleteDevWorkspace() {
  kubectl delete devworkspace ${DEV_WORKSPACE_NAME} -n ${USER_NAMESPACE}
}