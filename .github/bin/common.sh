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

export ECLIPSE_CHE_PACKAGE_NAME="eclipse-che-preview-openshift"
export ECLIPSE_CHE_CATALOG_SOURCE_NAME="eclipse-che-custom-catalog-source"
export ECLIPSE_CHE_SUBSCRIPTION_NAME="eclipse-che-subscription"
export DEV_WORKSPACE_CATALOG_SOURCE_NAME="custom-devworkspace-operator-catalog"

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
  make gen-chectl-tmpl SOURCE="${PREVIOUS_OPERATOR_VERSION_CLONE_PATH}" TARGET="${PREVIOUS_OPERATOR_VERSION_TEMPLATE_PATH}"
  make gen-chectl-tmpl SOURCE="${LAST_OPERATOR_VERSION_CLONE_PATH}" TARGET="${LAST_OPERATOR_VERSION_TEMPLATE_PATH}"
  make gen-chectl-tmpl SOURCE="${OPERATOR_REPO}" TARGET="${CURRENT_OPERATOR_VERSION_TEMPLATE_PATH}"
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

buildCheOperatorImage() {
  docker build -t "${OPERATOR_IMAGE}" -f Dockerfile --build-arg SKIP_TESTS=true . && docker save "${OPERATOR_IMAGE}" > /tmp/operator.tar
}

copyCheOperatorImageToMinikube() {
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
    yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${OPERATOR_DEPLOYMENT}"
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
  waitEclipseCheDeployed "${cheVersion}"

  waitDevWorkspaceControllerStarted
}

deployEclipseCheWithOperator() {
  local chectlbin=$1
  local platform=$2
  local templates=$3
  local customimage=$4

  if [[ ${customimage} == "true"  ]]; then
    if [[ ${platform} == "minikube" ]]; then
      buildCheOperatorImage
      copyCheOperatorImageToMinikube
      yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${templates}"/che-operator/kubernetes/operator.yaml
      yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${templates}"/che-operator/kubernetes/operator.yaml
    else
      yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${templates}"/che-operator/openshift/operator.yaml
      yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' "${templates}"/che-operator/openshift/operator.yaml
    fi
  fi

  if [[ ${platform} == "minikube" ]]; then
    checluster=$(grep -rlx "kind: CheCluster" /tmp/chectl-templates/che-operator/)
    apiVersion=$(yq -r '.apiVersion' ${checluster})
    if [[ ${apiVersion} == "org.eclipse.che/v2" ]]; then
      yq -riY '.spec.networking.domain = "'$(minikube ip).nip.io'"' ${checluster}
      yq -riY '.spec.networking.tlsSecretName = "che-tls"' ${checluster}
    else
      yq -riY '.spec.k8s.ingressDomain = "'$(minikube ip).nip.io'"' ${checluster}
    fi
  fi

  ${chectlbin} server:deploy \
    --batch \
    --platform ${platform} \
    --installer operator \
    --templates ${templates}

  waitDevWorkspaceControllerStarted
}

updateEclipseChe() {
  local chectlbin=$1
  local platform=$2
  local templates=$3
  local customimage=$4

  if [[ ${customimage} == "true"  ]]; then
    if [[ ${platform} == "minikube" ]]; then
      buildCheOperatorImage
      copyCheOperatorImageToMinikube
      yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${templates}/che-operator/kubernetes/operator.yaml
      yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' ${templates}/che-operator/kubernetes/operator.yaml
    else
      yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' ${templates}/che-operator/openshift/operator.yaml
      yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' ${templates}/che-operator/openshift/operator.yaml
    fi
  fi

  ${chectlbin} server:update \
    --batch \
    --templates ${templates}

  local configManagerPath=""
  if [[ -f ${templates}/che-operator/operator.yaml ]]; then
    configManagerPath="${templates}/che-operator/operator.yaml"
  elif [[ ${platform} == "minikube" ]]; then
    configManagerPath="${templates}/che-operator/kubernetes/operator.yaml"
  else
    configManagerPath="${templates}/che-operator/openshift/operator.yaml"
  fi

  local cheVersion=$(cat "${configManagerPath}" | yq -r '.spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value')
  waitEclipseCheDeployed ${cheVersion}

  waitDevWorkspaceControllerStarted
}

waitEclipseCheDeployed() {
  local version=$1
  echo "[INFO] Wait for Eclipse Che '${version}' version"

  export n=0
  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.cheVersion}")
    chePhase=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o "jsonpath={.status.chePhase}" )
    oc get pods -n ${NAMESPACE}
    if [[ "${cheVersion}" == "${version}" ]]
    then
      echo "[INFO] Eclipse Che '${version}' version has been successfully deployed"
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
  oc patch csv $(getCSVName) -n openshift-operators --type=json -p '[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "'${image}'"}]'
}

getCheClusterCRFromExistedCSV() {
  CHE_CLUSTER=""
  CHE_CLUSTER_V2=$(oc get csv $(getCSVName) -n openshift-operators -o yaml | yq -r '.metadata.annotations["alm-examples"] | fromjson | .[] | select(.apiVersion == "org.eclipse.che/v2")')
  if [[ -n "${CHE_CLUSTER_V2}" ]]; then
    CHE_CLUSTER="${CHE_CLUSTER_V2}"
  else
    CHE_CLUSTER_V1=$(oc get csv $(getCSVName) -n openshift-operators -o yaml | yq -r '.metadata.annotations["alm-examples"] | fromjson | .[] | select(.apiVersion == "org.eclipse.che/v1")')
    CHE_CLUSTER="${CHE_CLUSTER_V1}"
  fi

  echo "${CHE_CLUSTER}"
}

getCheVersionFromExistedCSV() {
  oc get csv $(getCSVName) -n openshift-operators -o yaml | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env[] | select(.name == "CHE_VERSION") | .value'
}

getCSVName() {
  local n=0
  local csvNumber=0

  while [ $n -le 24 ]
  do
    csvNumber=$(oc get csv -n openshift-operators --no-headers=true | grep ${ECLIPSE_CHE_PACKAGE_NAME} | wc -l)
    if [[ $csvNumber == 1 ]]; then
      break
      return
    fi

    sleep 5
    n=$(( n+1 ))
  done

  if [[ $csvNumber != 1 ]]; then
    echo "[ERROR] More than 1 Eclipse Che CSV found"
    exit 1
  fi

  oc get csv -n openshift-operators | grep eclipse-che-preview-openshift | awk '{print $1}'
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

  sleep 20s
  kubectl wait --for=condition=ready pod -l "olm.catalogSource=${name}" -n openshift-operators --timeout=240s
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
  if [[ ${installPlan} == "Manual" ]]; then
    kubectl wait subscription/"${name}" -n openshift-operators --for=condition=InstallPlanPending --timeout=120s
  fi
}

deployDevWorkspaceOperator() {
  echo "[INFO] Deploy Dev Workspace operator"

  local cheChannel=${1}
  local devWorkspaceCatalogImage="quay.io/devfile/devworkspace-operator-index:next"
  local devWorkspaceChannel="next"

  if [[ ${cheChannel} == "stable" ]]; then
    devWorkspaceCatalogImage="quay.io/devfile/devworkspace-operator-index:release"
    devWorkspaceChannel="fast"
  fi

  createCatalogSource "${DEV_WORKSPACE_CATALOG_SOURCE_NAME}" ${devWorkspaceCatalogImage} "Red Hat" "DevWorkspace Operator Catalog"
  createSubscription "devworkspace-operator" "devworkspace-operator" "${devWorkspaceChannel}" "${DEV_WORKSPACE_CATALOG_SOURCE_NAME}" "Auto"

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

  yq -r "(.spec.template.spec.containers[0].image) = \"${image}\"" "${OPERATOR_REPO}/olm/force-pulling-olm-images-job.yaml" | kubectl apply -f - -n openshift-operators

  kubectl wait --for=condition=complete --timeout=30s job/force-pulling-olm-images-job -n openshift-operators
  kubectl delete job/force-pulling-olm-images-job -n openshift-operators
}

installchectl() {
  local version=$1
  curl -L https://github.com/che-incubator/chectl/releases/download/${version}/chectl-linux-x64.tar.gz -o /tmp/chectl-${version}.tar.gz
  rm -rf /tmp/chectl-${version}
  mkdir /tmp/chectl-${version}
  tar -xvzf /tmp/chectl-${version}.tar.gz -C /tmp/chectl-${version}
}

getBundlePath() {
  channel="${1}"
  if [ -z "${channel}" ]; then
    echo "[ERROR] 'channel' is not specified"
    exit 1
  fi

  echo "${OPERATOR_REPO}/bundle/${channel}/${ECLIPSE_CHE_PACKAGE_NAME}"
}
