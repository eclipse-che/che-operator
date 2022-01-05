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
  set -x
  local version=$1
  echo "[INFO] Wait for Eclipse Che '${version}' version"

  export n=0
  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheVersion}")
    cheIsRunning=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheClusterRunning}" )
    oc get pods -n ${NAMESPACE}
    if [ "${cheVersion}" == "${version}" ] && [ "${cheIsRunning}" == "Available" ]
    then
      echo "[INFO] Eclipse Che '${version}' version has been succesfully deployed"
      break
    fi
    sleep 6
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "[ERROR] Failed to deploy Eclipse Che '${version}' verion"
    exit 1
  fi
}

useCustomOperatorImageInCSV() {
  local image=$1
  oc patch csv $(getCSVName) -n ${NAMESPACE} --type=json -p '[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "'${image}'"}]'
}

getCheClusterCRFromExistedCSV() {
  oc get csv $(getCSVName) -n ${NAMESPACE} -o yaml | yq -r ".metadata.annotations[\"alm-examples\"] | fromjson | .[] | select(.kind == \"CheCluster\")"
}

getCheVersionFromExistedCSV() {
  oc get csv $(getCSVName) -n ${NAMESPACE} -o yaml | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value'
}

getCSVName() {
  oc get csv -n ${NAMESPACE} | grep eclipse-che-preview-openshift | awk '{print $1}'
}

waitDevWorkspaceControllerStarted() {
  echo "[INFO] Wait for Dev Workspace controller started"

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

  sleep 15s
  kubectl wait --for=condition=ready pod -l olm.catalogSource=community-catalog -n openshift-marketplace --timeout=120s
}

createCatalogSource() {
  local name="${1}"
  local image="${2}"
  local publisher="${3:-Eclipse-Che}"
  local displayName="${4:-Eclipse Che Operator Catalog}"

  echo "[INFO] Create catalog source '${name}' with image '${image}'"

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${name}
  namespace: openshift-operators
spec:
  sourceType: grpc
  publisher: ${publisher}
  displayName: ${displayName}
  image: ${image}
  updateStrategy:
    registryPoll:
      interval: 15m
EOF

  sleep 10s
  kubectl wait --for=condition=ready pod -l "olm.catalogSource=${name}" -n openshift-operators --timeout=120s
}

createSubscription() {
  local name=${1}
  local packageName=${2}
  local channel=${3}
  local source=${4}
  local installPlan=${5}
  local startingCSV=${6}

  echo "[INFO] Create subscription '${name}'"

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${name}
  namespace: openshift-operators
spec:
  channel: ${channel}
  installPlanApproval: ${installPlan}
  name: ${packageName}
  source: ${source}
  sourceNamespace: openshift-operators
  startingCSV: ${startingCSV}
EOF

  sleep 10s
  if [[ ${installPlan} == "Manual"} ]]; then
    kubectl wait subscription/"${packageName}" -n openshift-operators --for=condition=InstallPlanPending --timeout=120s
  fi
}

deployDevWorkspaceOperatorFromFastChannel() {
  echo "[INFO] Deploy Dev Workspace operator from 'fast' channel"

  customDevWorkspaceCatalog=$(getDevWorkspaceCustomCatalogSourceName)
  createCatalogSource "${customDevWorkspaceCatalog}" "quay.io/devfile/devworkspace-operator-index:next"
  createSubscription "devworkspace-operator" "devworkspace-operator" "fast" "${customDevWorkspaceCatalog}" "Auto"

  waitDevWorkspaceControllerStarted
}

deployDevWorkspaceOperatorFromNextChannel() {
  echo "[INFO] Deploy Dev Workspace operator from 'stable' channel"

  customDevWorkspaceCatalog=$(getDevWorkspaceCustomCatalogSourceName)
  createCatalogSource "${customDevWorkspaceCatalog}" "quay.io/devfile/devworkspace-operator-index:next" "Red Hat" "DevWorkspace Operator Catalog"
  createSubscription "devworkspace-operator" "devworkspace-operator" "fast" "${customDevWorkspaceCatalog}" "Auto"

  waitDevWorkspaceControllerStarted
}

createNamespace() {
  namespace="${1}"

  echo "[INFO] Create namespace '${namespace}'"

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
EOF
}

approveInstallPlan() {
  local name="${1}"

  echo "[INFO] Approve install plan '${name}'"

  local installPlan=$(kubectl get subscription/${name} -n openshift-operators -o jsonpath='{.status.installplan.name}')
  kubectl patch installplan/${installPlan} -n openshift-operators --type=merge -p '{"spec":{"approved":true}}'
  kubectl wait installplan/${installPlan} -n openshift-operators --for=condition=Installed --timeout=240s
}

getCatalogSourceBundles() {
  local name=${1}
  local catalogService=$(kubectl get service "${name}" -n openshift-operators -o yaml)
  local catalogIP=$(echo "${catalogService}" | yq -r ".spec.clusterIP")
  local catalogPort=$(echo "${catalogService}" | yq -r ".spec.ports[0].targetPort")

  LIST_BUNDLES=$(kubectl run grpcurl-query -n openshift-operators \
  --rm=true \
  --restart=Never \
  --attach=true \
  --image=docker.io/fullstorydev/grpcurl:v1.7.0 \
  --  -plaintext "${catalogIP}:${catalogPort}" api.Registry.ListBundles
  )

  echo "${LIST_BUNDLES}" | head -n -1
}

fetchPreviousCSVInfo() {
  local channel="${1}"
  local bundles="${2}"

  previousBundle=$(echo "${bundles}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 2]')
  export PREVIOUS_CSV_NAME=$(echo "${previousBundle}" | yq -r ".csvName")
  if [ "${PREVIOUS_CSV_NAME}" == "null" ]; then
    echo "[ERROR] Catalog source image hasn't got previous bundle."
    exit 1
  fi
  export PREVIOUS_CSV_BUNDLE_IMAGE=$(echo "${previousBundle}" | yq -r ".bundlePath")
}

fetchLatestCSVInfo() {
  local channel="${1}"
  local bundles="${2}"

  latestBundle=$(echo "${bundles}" | jq -s '.' | jq ". | map(. | select(.channelName == \"${channel}\"))" | yq -r '. |=sort_by(.csvName) | .[length - 1]')
  export LATEST_CSV_NAME=$(echo "${latestBundle}" | yq -r ".csvName")
  export LATEST_CSV_BUNDLE_IMAGE=$(echo "${latestBundle}" | yq -r ".bundlePath")
}

# HACK. Unfortunately catalog source image bundle job has image pull policy "IfNotPresent".
# It makes troubles for test scripts, because image bundle could be outdated with
# such pull policy. That's why we launch job to fource image bundle pulling before Che installation.
forcePullingOlmImages() {
  image="${1}"

  echo "[INFO] Pulling image '${image}'"

  yq -r "(.spec.template.spec.containers[0].image) = \"${image}\"" "${BASE_DIR}/force-pulling-olm-images-job.yaml" | kubectl apply -f - -n openshift-operators

  kubectl wait --for=condition=complete --timeout=30s job/force-pulling-olm-images-job -n openshift-operators
  kubectl delete job/force-pulling-olm-images-job -n openshift-operators
}
