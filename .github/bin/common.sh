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

set -e
set -x

catchFinish() {
  result=$?

  collectLogs
  if [ "$result" != "0" ]; then
    echo "[ERROR] Job failed."
  else
    echo "[INFO] Job completed successfully."
  fi

  echo "[INFO] Please check github actions artifacts."
  exit $result
}

initDefaults() {
  export NAMESPACE="eclipse-che"
  export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/artifacts-che"}
  export CHECTL_TEMPLATES_BASE_DIR=/tmp/chectl-templates
  export OPERATOR_IMAGE="test/che-operator:test"
  export IS_OPENSHIFT=$(kubectl api-resources --api-group="route.openshift.io" --no-headers=true | head -n1 | wc -l)
  export IS_KUBERNETES=$(if [[ $IS_OPENSHIFT == 0 ]]; then echo 1; else echo 0; fi)

  # turn off telemetry
  mkdir -p ${HOME}/.config/chectl
  echo "{\"segment.telemetry\":\"off\"}" > ${HOME}/.config/chectl/config.json

  getLatestsStableVersions
}

initTemplates() {
  rm -rf ${OPERATOR_REPO}/tmp
  rm -rf ${CHECTL_TEMPLATES_BASE_DIR} && mkdir -p ${CHECTL_TEMPLATES_BASE_DIR} && chmod 777 ${CHECTL_TEMPLATES_BASE_DIR}

  PREVIOUS_OPERATOR_VERSION_CLONE_PATH=${OPERATOR_REPO}/tmp/${PREVIOUS_PACKAGE_VERSION}
  git clone --depth 1 --branch ${PREVIOUS_PACKAGE_VERSION} https://github.com/eclipse-che/che-operator/ ${PREVIOUS_OPERATOR_VERSION_CLONE_PATH}

  LAST_OPERATOR_VERSION_CLONE_PATH=${OPERATOR_REPO}/tmp/${LAST_PACKAGE_VERSION}
  git clone --depth 1 --branch ${LAST_PACKAGE_VERSION} https://github.com/eclipse-che/che-operator/ ${LAST_OPERATOR_VERSION_CLONE_PATH}

  export CURRENT_OPERATOR_VERSION_TEMPLATE_PATH=${CHECTL_TEMPLATES_BASE_DIR}
  export PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH=${CHECTL_TEMPLATES_BASE_DIR}/${PREVIOUS_PACKAGE_VERSION}
  export LAST_OPERATOR_VERSION_TEMPLATE_PATH=${CHECTL_TEMPLATES_BASE_DIR}/${LAST_PACKAGE_VERSION}

  copyChectlTemplates "${PREVIOUS_OPERATOR_VERSION_CLONE_PATH}" "${PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator"
  copyChectlTemplates "${LAST_OPERATOR_VERSION_CLONE_PATH}" "${LAST_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator"
  copyChectlTemplates "${OPERATOR_REPO}" "${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}/che-operator"
}

getLatestsStableVersions() {
  git remote add operator https://github.com/eclipse-che/che-operator.git
  git fetch operator -q
  tags=$(git ls-remote --refs --tags operator | sed -n 's|.*refs/tags/\(7.*\)|\1|p' | awk -F. '{ print ($1*1000)+($2*10)+$3" "$1"."$2"."$3}' | sort | tac)
  export PREVIOUS_PACKAGE_VERSION=$(echo "${tags}" | sed -n 2p | cut -d ' ' -f2)
  export LAST_PACKAGE_VERSION=$(echo "${tags}" | sed -n 1p | cut -d ' ' -f2)
}

copyChectlTemplates() {
  pushd "${OPERATOR_REPO}" || exit
  make chectl-templ "SRC=${1}" "TARGET=${2}"
  popd || exit
}

collectLogs() {
  mkdir -p ${ARTIFACTS_DIR}

  set +ex
  if [[ $IS_KUBERNETES == 1 ]]; then
    # Collect logs only for k8s cluster since OpenShift CI already dump all resources
    collectClusterResources
  fi

  # additionally grab server logs for fast access
  chectl server:logs -n $NAMESPACE -d $ARTIFACTS_DIR
  set -ex
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

buildCheOperatorImage() {
  docker build -t "${OPERATOR_IMAGE}" -f Dockerfile --build-arg TESTS=false . && docker save "${OPERATOR_IMAGE}" > /tmp/operator.tar
}

copyCheOperatorImageToMinikube() {
  eval $(minikube docker-env) && docker load -i  /tmp/operator.tar && rm  /tmp/operator.tar
}

deployEclipseCheOnWithOperator() {
  local platform=$1
  local templates=$2
  local customimage=$3

  local domainFlag=""
  if [[ ${platform} == "minikube" ]]; then
    domainFlag="--domain $(minikube ip).nip.io"
  fi

  if [[ ${customimage} == "true"  ]]; then
    if [[ ${platform} == "minikube" ]]; then
      buildCheOperatorImage
      copyCheOperatorImageToMinikube
    fi

    yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${templates}/che-operator/operator.yaml
    yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' ${templates}/che-operator/operator.yaml
  fi

  chectl server:deploy \
    --batch \
    --platform ${platform} \
    --installer operator \
    --workspace-engine dev-workspace \
    --templates ${templates} ${domainFlag}

  waitDevWorkspaceControllerStarted
}

updateEclipseChe() {
  local platform=$1
  local templates=$2
  local customimage=$3

  if [[ ${customimage} == "true"  ]]; then
    if [[ ${platform} == "minikube" ]]; then
      buildCheOperatorImage
      copyCheOperatorImageToMinikube
    fi

    yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${templates}/che-operator/operator.yaml
    yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' ${templates}/che-operator/operator.yaml
  fi

  chectl server:update \
    --batch \
    --templates ${templates}

  local cheVersion=$(cat ${templates}/che-operator/operator.yaml | yq -r '.spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value')
  waitEclipseCheDeployed ${cheVersion}

  waitDevWorkspaceControllerStarted
}

waitEclipseCheDeployed() {
  local version=$1
  export n=0

  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheVersion}")
    cheIsRunning=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheClusterRunning}" )
    oc get pods -n ${NAMESPACE}
    if [ "${cheVersion}" == "${version}" ] && [ "${cheIsRunning}" == "Available" ]
    then
      echo "Eclipse Che ${version} has been succesfully deployed"
      break
    fi
    sleep 6
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "Failed to deploy Eclipse Che ${version}"
    exit 1
  fi
}

useCustomOperatorImageInCSV() {
  local image=$1
  oc patch csv $(getCSVName) -n openshift-operators --type=json -p '[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "'${image}'"}]'
}

createEclipseCheCRFromCSV() {
  oc get csv $(getCSVName) -n openshift-operators -o yaml | yq -r ".metadata.annotations[\"alm-examples\"] | fromjson | .[] | select(.kind == \"CheCluster\")" | oc apply -n "${NAMESPACE}" -f -
}

getCSVName() {
  echo $(oc get csv -n openshift-operators | grep eclipse-che-preview-openshift | awk '{print $1}')
}

waitDevWorkspaceControllerStarted() {
  n=0
  while [ $n -le 24 ]
  do
    webhooks=$(oc get mutatingWebhookConfiguration --all-namespaces)
    if [[ $webhooks =~ .*controller.devfile.io.* ]]; then
      echo "[INFO] Dev Workspace controller has been deployed"
      return
    fi

    sleep 5
    n=$(( n+1 ))
  done

  echo "[ERROR] Failed to deploy Dev Workspace controller"
  exit 1
}

deployCommunityCatalog() {
  oc create -f - -o jsonpath='{.metadata.name}' <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: community-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: registry.redhat.io/redhat/community-operator-index:v4.9
  displayName: Eclipse Che Catalog
  publisher: Eclipse Che
  updateStrategy:
    registryPoll:
      interval: 30m
EOF
  sleep 10s
  kubectl wait --for=condition=ready pod -l olm.catalogSource=community-catalog -n openshift-marketplace --timeout=120s
}
