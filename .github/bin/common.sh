#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
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
  export RAM_MEMORY=8192
  export NAMESPACE="eclipse-che"
  export USER_NAMEPSACE="admin-che"
  export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/artifacts-che"}
  export TEMPLATES=${OPERATOR_REPO}/tmp
  export OPERATOR_IMAGE="test/che-operator:test"
  export DEFAULT_DEVFILE="https://raw.githubusercontent.com/eclipse-che/che-devfile-registry/main/devfiles/nodejs/devfile.yaml"
  export CHE_EXPOSURE_STRATEGY="multi-host"
  export OPENSHIFT_NEXT_CSV_FILE="${OPERATOR_REPO}/bundle/next/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"
  export DEV_WORKSPACE_CONTROLLER_VERSION="main"
  export DEV_WORKSPACE_ENABLE="false"
  export DEVWORKSPACE_CONTROLLER_TEST_NAMESPACE=devworkspace-controller-test
  export DEVWORKSPACE_CHE_OPERATOR_TEST_NAMESPACE=devworkspace-cheoperator-test
  export IMAGE_PULLER_ENABLE="false"

  # turn off telemetry
  mkdir -p ${HOME}/.config/chectl
  echo "{\"segment.telemetry\":\"off\"}" > ${HOME}/.config/chectl/config.json

  # prepare templates directory
  rm -rf ${TEMPLATES}
  mkdir -p "${TEMPLATES}/che-operator" && chmod 777 "${TEMPLATES}"
}

initLatestTemplates() {
rm -rf /tmp/devfile-devworkspace-operator-*
curl -L https://api.github.com/repos/devfile/devworkspace-operator/zipball/${DEV_WORKSPACE_CONTROLLER_VERSION} > /tmp/devworkspace-operator.zip && \
  unzip /tmp/devworkspace-operator.zip */deploy/deployment/* -d /tmp && \
  mkdir -p /tmp/devworkspace-operator/templates/ && \
  mv /tmp/devfile-devworkspace-operator-*/deploy ${TEMPLATES}/devworkspace

  prepareTemplates "${OPERATOR_REPO}" "${TEMPLATES}/che-operator"
}

getLatestsStableVersions() {
  # Get Stable and new release versions from olm files openshift.
  versions=$(curl \
  -H "Authorization: bearer ${GITHUB_TOKEN}" \
  -X POST -H "Content-Type: application/json" --data \
  '{"query": "{ repository(owner: \"eclipse-che\", name: \"che-operator\") { refs(refPrefix: \"refs/tags/\", last: 2, orderBy: {field: TAG_COMMIT_DATE, direction: ASC}) { edges { node { name } } } } }" } ' \
  https://api.github.com/graphql)

  echo "${versions[*]}"

  LAST_PACKAGE_VERSION=$(echo "${versions[@]}" | jq '.data.repository.refs.edges[1].node.name | sub("\""; "")' | tr -d '"')
  export LAST_PACKAGE_VERSION
  PREVIOUS_PACKAGE_VERSION=$(echo "${versions[@]}" | jq '.data.repository.refs.edges[0].node.name | sub("\""; "")' | tr -d '"')
  export PREVIOUS_PACKAGE_VERSION
}

initStableTemplates() {
  getLatestsStableVersions

  export lastOperatorPath=${OPERATOR_REPO}/tmp/${LAST_PACKAGE_VERSION}
  export previousOperatorPath=${OPERATOR_REPO}/tmp/${PREVIOUS_PACKAGE_VERSION}

  export LAST_OPERATOR_TEMPLATE=${lastOperatorPath}/chectl/templates
  export PREVIOUS_OPERATOR_TEMPLATE=${previousOperatorPath}/chectl/templates

  # clone the exact versions to use their templates
  git clone --depth 1 --branch ${PREVIOUS_PACKAGE_VERSION} https://github.com/eclipse-che/che-operator/ ${previousOperatorPath}
  git clone --depth 1 --branch ${LAST_PACKAGE_VERSION} https://github.com/eclipse-che/che-operator/ ${lastOperatorPath}

  # chectl requires 'che-operator' template folder
  mkdir -p "${LAST_OPERATOR_TEMPLATE}/che-operator"
  mkdir -p "${PREVIOUS_OPERATOR_TEMPLATE}/che-operator"

  compareResult=$(pysemver compare "${LAST_PACKAGE_VERSION}" "7.34.0")
  if [ "${compareResult}" == "-1" ]; then
    cp -rf ${lastOperatorPath}/deploy/* "${LAST_OPERATOR_TEMPLATE}/che-operator"
  else
    prepareTemplates "${lastOperatorPath}" "${LAST_OPERATOR_TEMPLATE}/che-operator"
  fi
}

# Utility to wait for a workspace to be started after workspace:create.
waitWorkspaceStart() {
  export x=0
  timeout=240
  while [ $x -le $timeout ]
  do
    login

    chectl workspace:list --chenamespace=${NAMESPACE}
    workspaceStatus=$(chectl workspace:list --chenamespace=${NAMESPACE} | tail -1 | awk '{ print $4} ')

    if [ "${workspaceStatus}" == "RUNNING" ]; then
      echo "[INFO] Workspace started successfully"
      break
    elif [ "${workspaceStatus}" == "STOPPED" ]; then
      echo "[ERROR] Workspace failed to start"
      exit 1
    fi
    sleep 10
    x=$(( x+1 ))
  done

  if [ $x -gt $timeout ]
  then
    echo "[ERROR] Workspace didn't start after 4 minutes."
    exit 1
  fi
}

waitExistedWorkspaceStop() {
  login

  maxAttempts=10
  count=0
  while [ $count -le $maxAttempts ]; do
    chectl workspace:list --chenamespace=${NAMESPACE}
    workspaceStatus=$(chectl workspace:list --chenamespace=${NAMESPACE} | tail -1 | awk '{ print $4} ')

    if [ "${workspaceStatus}" == "STOPPED" ]; then
      echo "[INFO] Workspace stopped successfully"
      break
    fi

    if [ $x -gt $maxAttempts ]; then
      echo "[ERROR] Filed to stop workspace"
      exit 1
    fi

    sleep 10
    count=$((count+1))
  done
}

installYq() {
  YQ=$(command -v yq) || true
  if [[ ! -x "${YQ}" ]]; then
    pip3 install wheel
    pip3 install yq
  fi
  echo "[INFO] $(yq --version)"
  echo "[INFO] $(jq --version)"
}

# Graps Eclipse Che logs
collectLogs() {
  mkdir -p ${ARTIFACTS_DIR}

  set +e
  chectl server:logs --chenamespace=${NAMESPACE} --directory=${ARTIFACTS_DIR}
  collectClusterData
  collectDevworkspaceOperatorLogs
  set -e
}

collectClusterData() {
  allNamespaces=$(kubectl get namespaces -o custom-columns=":metadata.name")
  for namespace in $allNamespaces ; do
    collectK8sResourcesForNamespace $namespace
    collectPodsLogsForNamespace $namespace
  done
  collectClusterScopeK8sResources
}

collectK8sResourcesForNamespace() {
  namespace="$1"
  if [[ -z $namespace ]]; then return; fi

  declare -a KINDS=("pods" "jobs" "deployments"
                    "services" "ingresses"
                    "configmaps" "secrets"
                    "serviceaccounts" "roles" "rolebindings"
                    "events"
                    "pv" "pvc"
                    "checlusters" "checlusterbackups" "checlusterrestores" "chebackupserverconfigurations"
                   )
  for kind in "${KINDS[@]}" ; do
    dir="${ARTIFACTS_DIR}/cluster/namespaces/${namespace}/${kind}"
    mkdir -p $dir

    names=$(kubectl get -n $namespace $kind --no-headers=true -o custom-columns=":metadata.name")
    for name in $names ; do
      name=${name//[:<>|*?]/_}
      kubectl get -n $namespace $kind $name -o yaml > "${dir}/${name}.yaml"
    done
  done
}

collectClusterScopeK8sResources() {
  declare -a KINDS=("crds"
                    "clusterroles" "clusterrolebindings"
                   )
  for kind in "${KINDS[@]}" ; do
    dir="${ARTIFACTS_DIR}/cluster/global/${kind}"
    mkdir -p $dir

    names=$(kubectl get -n $namespace $kind --no-headers=true -o custom-columns=":metadata.name")
    for name in $names ; do
      name=${name//[:<>|*?]/_}
      kubectl get -n $namespace $kind $name -o yaml > "${dir}/${name}.yaml"
    done
  done
}

collectPodsLogsForNamespace() {
  namespace="$1"
  if [[ -z $namespace ]]; then return; fi

  dir="${ARTIFACTS_DIR}/cluster/namespaces/${namespace}/logs"
  mkdir -p $dir

  pods=$(kubectl get -n $namespace pods --no-headers=true -o custom-columns=":metadata.name")
  for pod in $pods ; do
    kubectl logs -n $namespace $pod > "${dir}/${pod}.log"
  done
}

collectDevworkspaceOperatorLogs() {
  mkdir -p ${ARTIFACTS_DIR}/devworkspace-operator

  oc get events -n devworkspace-controller > ${ARTIFACTS_DIR}/events-devworkspace-controller.txt

  #determine the name of the devworkspace controller manager pod
  local CONTROLLER_POD_NAME=$(oc get pods -n devworkspace-controller -l app.kubernetes.io/name=devworkspace-controller -o json | jq -r '.items[0].metadata.name')
  local WEBHOOK_SVR_POD_NAME=$(oc get pods -n devworkspace-controller -l app.kubernetes.io/name=devworkspace-webhook-server -o json | jq -r '.items[0].metadata.name')

  # save the logs of all the containers in the DWO pod
  for container in $(oc get pod -n devworkspace-controller ${CONTROLLER_POD_NAME} -o json | jq -r '.spec.containers[] | .name'); do
    mkdir -p ${ARTIFACTS_DIR}/devworkspace-operator/${CONTROLLER_POD_NAME}
    oc logs -n devworkspace-controller deployment/devworkspace-controller-manager -c ${container} > ${ARTIFACTS_DIR}/devworkspace-operator/${CONTROLLER_POD_NAME}/${container}.log
  done

  for container in $(oc get pod -n devworkspace-controller ${WEBHOOK_SVR_POD_NAME} -o json | jq -r '.spec.containers[] | .name'); do
    mkdir -p ${ARTIFACTS_DIR}/devworkspace-operator/${WEBHOOK_SVR_POD_NAME}
    oc logs -n devworkspace-controller deployment/devworkspace-webhook-server -c ${container} > ${ARTIFACTS_DIR}/devworkspace-operator/${WEBHOOK_SVR_POD_NAME}/${container}.log
  done
}

# Build latest operator image
buildCheOperatorImage() {
  #docker build -t "${OPERATOR_IMAGE}" -f Dockerfile .
  docker build -t "${OPERATOR_IMAGE}" -f Dockerfile . && docker save "${OPERATOR_IMAGE}" > /tmp/operator.tar
}

copyCheOperatorImageToMinikube() {
  #docker save "${OPERATOR_IMAGE}" | minikube ssh --native-ssh=false -- docker load
  eval $(minikube docker-env) && docker load -i  /tmp/operator.tar && rm  /tmp/operator.tar
}

copyCheOperatorImageToMinishift() {
  #docker save -o "${OPERATOR_IMAGE}" | minishift ssh "docker load"
  eval $(minishift docker-env) && docker load -i  /tmp/operator.tar && rm  /tmp/operator.tar
}

# Prepare chectl che-operator templates
prepareTemplates() {
  if [ -n "${1}" ]; then
    SRC_TEMPLATES="${1}"
  else
    echo "[ERROR] Specify templates original location"
    exit 1
  fi

  if [ -n "${2}" ]; then
    TARGET_TEMPLATES="${2}"
  else
    echo "[ERROR] Specify templates target location"
    exit 1
  fi

  mkdir -p "${SRC_TEMPLATES}"

  cp -f "${SRC_TEMPLATES}/config/manager/manager.yaml" "${TARGET_TEMPLATES}/operator.yaml"

  cp -rf "${SRC_TEMPLATES}/config/crd/bases/" "${TARGET_TEMPLATES}/crds/"

  cp -f "${SRC_TEMPLATES}/config/rbac/role.yaml" "${TARGET_TEMPLATES}/"
  cp -f "${SRC_TEMPLATES}/config/rbac/role_binding.yaml" "${TARGET_TEMPLATES}/"
  cp -f "${SRC_TEMPLATES}/config/rbac/cluster_role.yaml" "${TARGET_TEMPLATES}/"
  cp -f "${SRC_TEMPLATES}/config/rbac/cluster_rolebinding.yaml" "${TARGET_TEMPLATES}/"
  cp -f "${SRC_TEMPLATES}/config/rbac/service_account.yaml" "${TARGET_TEMPLATES}/"

  cp -f "${SRC_TEMPLATES}/config/samples/org.eclipse.che_v1_checluster.yaml" "${TARGET_TEMPLATES}/crds/org_v1_che_cr.yaml"
  cp -f "${SRC_TEMPLATES}/config/crd/bases/org_v1_che_crd-v1beta1.yaml" "${TARGET_TEMPLATES}/crds/org_v1_che_crd-v1beta1.yaml"
}

deployEclipseCheStable(){
  local installer=$1
  local platform=$2
  local version=$3

  chectl server:deploy \
    --batch \
    --platform=${platform} \
    --installer ${installer} \
    --chenamespace ${NAMESPACE} \
    --skip-kubernetes-health-check \
    --version=${version}
}

deployEclipseCheWithTemplates() {
  local installer=$1
  local platform=$2
  local image=$3
  local templates=$4

  echo "[INFO] Eclipse Che custom resource"
  local crSample=${templates}/che-operator/crds/org_v1_che_cr.yaml
  cat ${crSample}

  echo "[INFO] Eclipse Che operator deployment"
  cat ${templates}/che-operator/operator.yaml

  chectl server:deploy \
    --batch \
    --platform=${platform} \
    --installer ${installer} \
    --chenamespace ${NAMESPACE} \
    --che-operator-image ${image} \
    --skip-kubernetes-health-check \
    --che-operator-cr-yaml ${crSample} \
    --templates ${templates}
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
      echo -e "\u001b[32m Eclipse Che ${version} has been succesfully deployed \u001b[0m"
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

updateEclipseChe() {
  local image=$1
  local templates=$2

  chectl server:update \
    --batch \
    --chenamespace=${NAMESPACE} \
    --che-operator-image=${image} \
    --templates=${templates}
}

# Create and start a workspace
startNewWorkspace() {
  sleep 5s
  login
  chectl workspace:create --start --chenamespace=${NAMESPACE} --devfile="${DEFAULT_DEVFILE}"
}

getOnlyWorkspaceId() {
  workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})

  # Grep applied to MacOS
  workspaceID=$(echo "$workspaceList" | grep workspace | awk '{ print $1} ')
  workspaceID="${workspaceID%'ID'}"

  echo $workspaceID
}

createWorkspace() {
  sleep 5s
  login
  chectl workspace:create --chenamespace=${NAMESPACE} --devfile="${DEFAULT_DEVFILE}"
}

startExistedWorkspace() {
  sleep 5s
  login

  workspaceID=$(getOnlyWorkspaceId)
  echo "[INFO] Workspace id of created workspace is: ${workspaceID}"

  chectl workspace:start $workspaceID
}

stopExistedWorkspace() {
  login

  workspaceID=$(getOnlyWorkspaceId)
  echo "[INFO] Workspace id of the workspace to stop is: ${workspaceID}"

  chectl workspace:stop $workspaceID
}

deleteExistedWorkspace() {
  login

  workspaceID=$(getOnlyWorkspaceId)
  echo "[INFO] Workspace id of the workspace to delete is: ${workspaceID}"

  chectl workspace:delete $workspaceID
}

disableOpenShiftOAuth() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.auth.openShiftoAuth = false' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

disableUpdateAdminPassword() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.auth.updateAdminPassword = false' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setServerExposureStrategy() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.server.serverExposureStrategy = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setSingleHostExposureType() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.k8s.singleHostExposureType = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setIngressDomain() {
  local file="${1}/che-operator/crds/org_v1_che_cr.yaml"
  yq -rSY '.spec.k8s.ingressDomain = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

setCustomOperatorImage() {
  local file="${1}/che-operator/operator.yaml"
  yq -rSY '.spec.template.spec.containers[0].image = "'${2}'"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
  yq -rSY '.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"' $file > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${file}
}

enableImagePuller() {
  kubectl patch checluster/eclipse-che -n ${NAMESPACE} --type=merge -p '{"spec":{"imagePuller":{"enable": true}}}'
}

insecurePrivateDockerRegistry() {
  IMAGE_REGISTRY_HOST="127.0.0.1:5000"
  export IMAGE_REGISTRY_HOST
}

# Utility to print objects created by Openshift CI automatically
printOlmCheObjects() {
  echo -e "[INFO] Operator Group object created in namespace: ${NAMESPACE}"
  oc get operatorgroup -n "${NAMESPACE}" -o yaml

  echo -e "[INFO] Catalog Source object created in namespace: ${NAMESPACE}"
  oc get catalogsource -n "${NAMESPACE}" -o yaml

  echo -e "[INFO] Subscription object created in namespace: ${NAMESPACE}"
  oc get subscription -n "${NAMESPACE}" -o yaml
}

# Patch subscription with image builded from source in Openshift CI job.
patchEclipseCheOperatorImage() {
  OPERATOR_POD=$(oc get pods -o json -n ${NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name')
  oc patch pod ${OPERATOR_POD} -n ${NAMESPACE} --type='json' -p='[{"op": "replace", "path": "/spec/containers/0/image", "value":'${OPERATOR_IMAGE}'}]'

  # The following command retrieve the operator image
  OPERATOR_POD_IMAGE=$(oc get pods -n ${NAMESPACE} -o json | jq -r '.items[] | select(.metadata.name | test("che-operator-")).spec.containers[].image')
  echo -e "[INFO] CHE operator image is ${OPERATOR_POD_IMAGE}"
}

# Create CheCluster object in Openshift ci with desired values
applyOlmCR() {
  echo "Creating Custom Resource"

  CR=$(yq -r ".metadata.annotations[\"alm-examples\"] | fromjson | .[] | select(.kind == \"CheCluster\")" "${OPENSHIFT_NEXT_CSV_FILE}")
  CR=$(echo "$CR" | yq -r ".spec.server.serverExposureStrategy = \"${CHE_EXPOSURE_STRATEGY}\"")
  CR=$(echo "$CR" | yq -r ".spec.devWorkspace.enable = ${DEV_WORKSPACE_ENABLE:-false}")
  CR=$(echo "$CR" | yq -r ".spec.imagePuller.enable = ${IMAGE_PULLER_ENABLE:-false}")

  echo -e "$CR"
  echo "$CR" | oc apply -n "${NAMESPACE}" -f -
}

# Create admin user inside of openshift cluster and login
function provisionOpenShiftOAuthUser() {
  oc create secret generic htpass-secret --from-file=htpasswd="${OPERATOR_REPO}"/.github/bin/resources/users.htpasswd -n openshift-config
  oc apply -f "${OPERATOR_REPO}"/.github/bin/resources/htpasswdProvider.yaml
  oc adm policy add-cluster-role-to-user cluster-admin user

  echo -e "[INFO] Waiting for htpasswd auth to be working up to 5 minutes"
  CURRENT_TIME=$(date +%s)
  ENDTIME=$(($CURRENT_TIME + 300))
  while [ $(date +%s) -lt $ENDTIME ]; do
      if oc login -u user -p user --insecure-skip-tls-verify=false; then
          break
      fi
      sleep 10
  done
}

login() {
  local oauth=$(kubectl get checluster eclipse-che -n $NAMESPACE -o json | jq -r '.spec.auth.openShiftoAuth')
  if [[ ${oauth} == "true" ]]; then
    # log in using OpenShift token
    chectl auth:login --chenamespace=${NAMESPACE}
  else
    chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
  fi
}

# Deploy Eclipse Che behind proxy in openshift ci
deployCheBehindProxy() {
  chectl server:deploy \
    --batch \
    --installer=operator \
    --platform=openshift \
    --templates=${TEMPLATES} \
    --che-operator-image ${OPERATOR_IMAGE}
  oc get checluster eclipse-che -n eclipse-che -o yaml
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

  OPERATOR_POD=$(oc get pods -o json -n ${NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name')
  oc logs ${OPERATOR_POD} -c che-operator -n ${NAMESPACE}

  exit 1
}

createWorkspaceDevWorkspaceController () {
  oc create namespace $DEVWORKSPACE_CONTROLLER_TEST_NAMESPACE
  sleep 10s

  echo -e "[INFO] Waiting for webhook-server to be running"
  CURRENT_TIME=$(date +%s)
  ENDTIME=$(($CURRENT_TIME + 180))
  while [ $(date +%s) -lt $ENDTIME ]; do
      if oc apply -f ${OPERATOR_REPO}/config/samples/devworkspace_flattened_theia-nodejs.yaml -n ${DEVWORKSPACE_CONTROLLER_TEST_NAMESPACE}; then
          break
      fi
      sleep 10
  done
}

waitAllPodsRunning() {
  echo "[INFO] Wait for running all pods"
  local namespace=$1

  n=0
  while [ $n -le 24 ]
  do
    pods=$(oc get pods -n ${namespace})
    if [[ $pods =~ .*Running.* ]]; then
      return
    fi

    kubectl get pods -n ${namespace}
    sleep 10
    n=$(( n+1 ))
  done

  echo "Failed to run pods in ${namespace}"
  exit 1
}

enableDevWorkspaceEngine() {
  kubectl patch checluster/eclipse-che -n ${NAMESPACE} --type=merge -p "{\"spec\":{\"server\":{\"customCheProperties\": {\"CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S\": \"true\"}}}}"
  kubectl patch checluster/eclipse-che -n ${NAMESPACE} --type=merge -p '{"spec":{"devWorkspace":{"enable": true}}}'
}

deployCertManager() {
  kubectl apply -f https://raw.githubusercontent.com/che-incubator/chectl/main/resources/cert-manager.yml
  sleep 10s

  kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=cert-manager -n cert-manager --timeout=60s
  kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=webhook -n cert-manager --timeout=60s
  kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=cainjector -n cert-manager --timeout=60s
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
  image: registry.redhat.io/redhat/community-operator-index:v4.7
  displayName: Eclipse Che Catalog
  publisher: Eclipse Che
  updateStrategy:
    registryPoll:
      interval: 30m
EOF
  sleep 10s
  kubectl wait --for=condition=ready pod -l olm.catalogSource=community-catalog -n openshift-marketplace --timeout=120s
}
