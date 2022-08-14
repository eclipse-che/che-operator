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
  export IS_OPENSHIFT=$(kubectl api-resources --api-group="route.openshift.io" --no-headers=true | head -n1 | wc -l)
  export IS_KUBERNETES=$(if [[ $IS_OPENSHIFT == 0 ]]; then echo 1; else echo 0; fi)

  # turn off telemetry
  mkdir -p ${HOME}/.config/chectl
  echo "{\"segment.telemetry\":\"off\"}" > ${HOME}/.config/chectl/config.json

  getLatestStableVersions
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

  pushd "${OPERATOR_REPO}" || exit 1
  make gen-chectl-tmpl SOURCE_PROJECT="${PREVIOUS_OPERATOR_VERSION_CLONE_PATH}" TEMPLATES="${PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH}"
  make gen-chectl-tmpl SOURCE_PROJECT="${LAST_OPERATOR_VERSION_CLONE_PATH}" TEMPLATES="${LAST_OPERATOR_VERSION_TEMPLATE_PATH}"
  make gen-chectl-tmpl SOURCE_PROJECT="${OPERATOR_REPO}" TEMPLATES="${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}"
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
  if [[ $IS_KUBERNETES == 1 ]]; then
    # Collect logs only for k8s cluster since OpenShift CI already dump all resources
    collectClusterResources
  fi

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

deployEclipseCheWithHelm() {
  local chectlbin=$1
  local platform=$2
  local templates=$3
  local customimage=$4
  local channel="next"

  # Deploy Eclipse Che to have Cert Manager and Dex installed
  deployEclipseCheWithOperator "${chectlbin}" "${platform}" "${templates}" "${customimage}"

  # Get configuration
  local identityProvider=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.identityProviderURL}')
  local oAuthSecret=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.oAuthSecret}')
  local oAuthClientName=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.oAuthClientName}')
  local domain=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.domain}')

  # Delete Eclipse Che (Cert Manager and Dex are still there)
  ${chectlbin} server:delete -y -n ${NAMESPACE}

  # Prepare HelmCharts
  HELMCHART_DIR=/tmp/chectl-helmcharts
  OPERATOR_DEPLOYMENT="${HELMCHART_DIR}"/templates/che-operator.Deployment.yaml

  rm -rf "${HELMCHART_DIR}" && cp -r "${OPERATOR_REPO}/helmcharts/${channel}" "${HELMCHART_DIR}"

  if [[ ${customimage} == "true"  ]]; then
    yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${OPERATOR_DEPLOYMENT}"
    yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "Never"' "${OPERATOR_DEPLOYMENT}"
  fi

  # Deploy Eclipse Che with Helm
  pushd "${HELMCHART_DIR}" || exit 1
  helm install che \
    --create-namespace \
    --namespace eclipse-che \
    --set networking.domain="${domain}" \
    --set networking.auth.oAuthSecret="${oAuthSecret}" \
    --set networking.auth.oAuthClientName="${oAuthClientName}" \
    --set networking.auth.identityProviderURL="${identityProvider}" .
  popd

  local cheVersion=$(yq -r '.spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value' < "${OPERATOR_DEPLOYMENT}")
  pushd ${OPERATOR_REPO}
    make wait-eclipseche-version VERSION="$(cheVersion)" NAMESPACE=${NAMESPACE}
    make wait-devworkspace-running NAMESPACE="devworkspace-controller"
  popd
}

deployEclipseCheWithOperator() {
  local chectlbin=$1
  local platform=$2
  local templates=$3
  local customimage=$4

  if [[ ${platform} == "minikube" ]]; then
    checluster=$(grep -rlx "kind: CheCluster" /tmp/chectl-templates/che-operator/)
    yq -riY '.spec.networking.domain = "'$(minikube ip).nip.io'"' ${checluster}
    yq -riY '.spec.networking.tlsSecretName = "che-tls"' ${checluster}

    if [[ ${customimage} == "true" ]]; then
      buildAndCopyCheOperatorImageToMinikube
      yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${templates}"/che-operator/kubernetes/operator.yaml
      yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${templates}"/che-operator/kubernetes/operator.yaml
    fi
  fi

  ${chectlbin} server:deploy \
    --batch \
    --platform ${platform} \
    --installer operator \
    --templates ${templates}

  pushd ${OPERATOR_REPO}
    make wait-devworkspace-running NAMESPACE="devworkspace-controller"
  popd
}

updateEclipseChe() {
  local chectlbin=$1
  local platform=$2
  local templates=$3
  local customimage=$4

  if [[ ${customimage} == "true"  ]]; then
    if [[ ${platform} == "minikube" ]]; then
      buildAndCopyCheOperatorImageToMinikube
      yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${templates}/che-operator/kubernetes/operator.yaml
      yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "Never"' ${templates}/che-operator/kubernetes/operator.yaml
    fi
  fi

  ${chectlbin} server:update \
    --batch \
    --templates ${templates}

  local configManagerPath="${templates}/che-operator/kubernetes/operator.yaml"
  local cheVersion=$(cat "${configManagerPath}" | yq -r '.spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value')

  pushd ${OPERATOR_REPO}
    make wait-eclipseche-version VERSION="$(cheVersion)" NAMESPACE=${NAMESPACE}
    make wait-devworkspace-running NAMESPACE="devworkspace-controller"
  popd
}

installchectl() {
  local version=$1
  curl -L https://github.com/che-incubator/chectl/releases/download/${version}/chectl-linux-x64.tar.gz -o /tmp/chectl-${version}.tar.gz
  rm -rf /tmp/chectl-${version}
  mkdir /tmp/chectl-${version}
  tar -xvzf /tmp/chectl-${version}.tar.gz -C /tmp/chectl-${version}
}
