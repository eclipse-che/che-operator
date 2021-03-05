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

  collectCheLogWithChectl
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
  export USER_NAMEPSACE="che-che"
  export ARTIFACTS_DIR="/tmp/artifacts-che"
  export TEMPLATES=${OPERATOR_REPO}/tmp
  export OPERATOR_IMAGE="test/che-operator:test"
  export DEFAULT_DEVFILE="https://raw.githubusercontent.com/eclipse/che-devfile-registry/master/devfiles/quarkus/devfile.yaml"
  export CHE_EXPOSURE_STRATEGY="multi-host"

  export XDG_DATA_HOME=/tmp/xdg_data
  export XDG_CACHE_HOME=/tmp/xdg_cache
  export XDG_CONFIG_HOME=/tmp/xdg_config

  export OAUTH="false"

  # turn off telemetry
  mkdir -p ${XDG_CONFIG_HOME}/chectl
  echo "{\"segment.telemetry\":\"off\"}" > ${XDG_CONFIG_HOME}/chectl/config.json

  # prepare templates directory
  rm -rf ${TEMPLATES}
  mkdir -p "${TEMPLATES}/che-operator" && chmod 777 "${TEMPLATES}"
}

initOpenShiftDefaults() {
  export OAUTH="true"
  export OPENSHIFT_NIGHTLY_CSV_FILE="${OPERATOR_REPO}/deploy/olm-catalog/eclipse-che-preview-openshift/manifests/che-operator.clusterserviceversion.yaml"
}

initLatestTemplates() {
  cp -rf ${OPERATOR_REPO}/deploy/* "${TEMPLATES}/che-operator"
}

initStableTemplates() {
  local platform=$1
  local channel=$2

  # Get Stable and new release versions from olm files openshift.
  export packageName=eclipse-che-preview-${platform}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
  export LAST_PACKAGE_VERSION=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")

  export previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${LAST_PACKAGE_VERSION}/${packageName}.v${LAST_PACKAGE_VERSION}.clusterserviceversion.yaml")
  export PREVIOUS_PACKAGE_VERSION=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

  export lastOperatorPath=${OPERATOR_REPO}/tmp/${LAST_PACKAGE_VERSION}
  export previousOperatorPath=${OPERATOR_REPO}/tmp/${PREVIOUS_PACKAGE_VERSION}

  export LAST_OPERATOR_TEMPLATE=${lastOperatorPath}/chectl/templates
  export PREVIOUS_OPERATOR_TEMPLATE=${previousOperatorPath}/chectl/templates

  # clone the exact versions to use their templates
  git clone --depth 1 --branch ${PREVIOUS_PACKAGE_VERSION} https://github.com/eclipse/che-operator/ ${previousOperatorPath}
  git clone --depth 1 --branch ${LAST_PACKAGE_VERSION} https://github.com/eclipse/che-operator/ ${lastOperatorPath}

  # chectl requires 'che-operator' template folder
  mkdir -p "${LAST_OPERATOR_TEMPLATE}/che-operator"
  mkdir -p "${PREVIOUS_OPERATOR_TEMPLATE}/che-operator"

  cp -rf ${previousOperatorPath}/deploy/* "${PREVIOUS_OPERATOR_TEMPLATE}/che-operator"
  cp -rf ${lastOperatorPath}/deploy/* "${LAST_OPERATOR_TEMPLATE}/che-operator"
}

# Utility to wait for a workspace to be started after workspace:create.
waitWorkspaceStart() {
  login

  set +e
  export x=0
  while [ $x -le 180 ]
  do
    chectl workspace:list --chenamespace=${NAMESPACE}
    workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})
    workspaceStatus=$(echo "$workspaceList" | grep RUNNING | awk '{ print $4} ')

    if [ "${workspaceStatus:-NOT_RUNNING}" == "RUNNING" ]
    then
      echo "[INFO] Workspace started successfully"
      break
    fi
    sleep 10
    x=$(( x+1 ))
  done

  if [ $x -gt 180 ]
  then
    echo "[ERROR] Workspace didn't start after 3 minutes."
    exit 1
  fi
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
collectCheLogWithChectl() {
  mkdir -p ${ARTIFACTS_DIR}
  chectl server:logs --chenamespace=${NAMESPACE} --directory=${ARTIFACTS_DIR}
}

# Build latest operator image
buildCheOperatorImage() {
  docker build -t "${OPERATOR_IMAGE}" -f Dockerfile . && docker save "${OPERATOR_IMAGE}" > operator.tar
}

copyCheOperatorImageToMinikube() {
  eval $(minikube docker-env) && docker load -i operator.tar && rm operator.tar
}

copyCheOperatorImageToMinishift() {
  eval $(minishift docker-env) && docker load -i operator.tar && rm operator.tar
}

deployEclipseChe() {
  local installer=$1
  local platform=$2
  local image=$3
  local templates=$4

  echo "[INFO] Eclipse Che custom resource"
  cat ${templates}/che-operator/crds/org_v1_che_cr.yaml

  echo "[INFO] Eclipse Che operator deployment"
  cat ${templates}/che-operator/operator.yaml

  chectl server:deploy \
    --platform=${platform} \
    --installer ${installer} \
    --chenamespace ${NAMESPACE} \
    --che-operator-image ${image} \
    --skip-kubernetes-health-check \
    --che-operator-cr-yaml ${templates}/che-operator/crds/org_v1_che_cr.yaml \
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

  chectl server:update --chenamespace=${NAMESPACE} -y --che-operator-image=${image} --templates=${templates}
}

startNewWorkspace() {
  # Create and start a workspace
  sleep 5s
  login
  chectl workspace:create --start --chenamespace=${NAMESPACE} --devfile="${DEFAULT_DEVFILE}"
}

createWorkspace() {
  sleep 5s
  login
  chectl workspace:create --chenamespace=${NAMESPACE} --devfile="${DEFAULT_DEVFILE}"
}

startExistedWorkspace() {
  sleep 5s
  login
  chectl workspace:list --chenamespace=${NAMESPACE}
  workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})

  # Grep applied to MacOS
  workspaceID=$(echo "$workspaceList" | grep workspace | awk '{ print $1} ')
  workspaceID="${workspaceID%'ID'}"
  echo "[INFO] Workspace id of created workspace is: ${workspaceID}"

  chectl workspace:start $workspaceID
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

insecurePrivateDockerRegistry() {
  IMAGE_REGISTRY_HOST="127.0.0.1:5000"
  export IMAGE_REGISTRY_HOST

  # local dockerDaemonConfig="/etc/docker/daemon.json"
  # sudo mkdir -p "/etc/docker"
  # sudo touch "${dockerDaemonConfig}"

  # config="{\"insecure-registries\" : [\"${IMAGE_REGISTRY_HOST}\"]}"
  # echo "${config}" | sudo tee "${dockerDaemonConfig}"

  # if [ -x "$(command -v docker)" ]; then
  #     echo "[INFO] Restart docker daemon to set up private registry info."
  #     sudo service docker restart
  # fi
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
patchEclipseCheOperatorSubscription() {
  OPERATOR_POD=$(oc get pods -o json -n ${NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name')
  oc patch pod ${OPERATOR_POD} -n ${NAMESPACE} --type='json' -p='[{"op": "replace", "path": "/spec/containers/0/image", "value":'${OPERATOR_IMAGE}'}]'

  # The following command retrieve the operator image
  OPERATOR_POD_IMAGE=$(oc get pods -n ${NAMESPACE} -o json | jq -r '.items[] | select(.metadata.name | test("che-operator-")).spec.containers[].image')
  echo -e "[INFO] CHE operator image is ${OPERATOR_POD_IMAGE}"
}

# Create CheCluster object in Openshift ci with desired values
applyOlmCR() {
  echo "Creating Custom Resource"

  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${OPENSHIFT_NIGHTLY_CSV_FILE}")
  CR=$(echo "$CRs" | yq -r ".[0]")
  CR=$(echo "$CR" | yq -r ".spec.auth.openShiftoAuth = ${OAUTH}")
  CR=$(echo "$CR" | yq -r ".spec.server.serverExposureStrategy = \"${CHE_EXPOSURE_STRATEGY}\"")

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
  if [[ ${OAUTH} == "false" ]]; then
    chectl auth:login -u admin -p admin --chenamespace=${NAMESPACE}
  else
    # log in using OpenShift token
    chectl auth:login --chenamespace=${NAMESPACE}
  fi
}

# Deploy Eclipse Che behind proxy in openshift ci
deployCheBehindProxy() {
  # Get the ocp domain for che custom resources
  export DOMAIN=$(oc get dns cluster -o json | jq .spec.baseDomain | sed -e 's/^"//' -e 's/"$//')

  # Related issue:https://github.com/eclipse/che/issues/17681
    cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  server:
    nonProxyHosts: oauth-openshift.apps.$DOMAIN|api.$DOMAIN
EOL

  chectl server:deploy --installer=operator --platform=openshift --batch --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml
  oc get checluster eclipse-che -n eclipse-che -o yaml
}

deployDevWorkspaceController() {
  oc patch checluster eclipse-che -n ${NAMESPACE}  --type=merge -p '{"spec":{"devWorkspace": {"enable": true}}}'
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
  oc logs ${OPERATOR_POD} -n ${NAMESPACE}

  exit 1
}

createWorksaceDevWorkspaceController () {
  oc apply -f https://raw.githubusercontent.com/devfile/devworkspace-operator/main/samples/flattened_theia-next.yaml -n ${NAMESPACE}
}

waitWorkspaceStartedDevWorkspaceController() {
  n=0
  while [ $n -le 24 ]
  do
    pods=$(oc get pods -n ${NAMESPACE})
    if [[ $pods =~ .*Running.* ]]; then
      echo "[INFO] Wokrspace started succesfully"
      return
    fi

    sleep 5
    n=$(( n+1 ))
  done

  echo "Failed to start a workspace"
  exit 1
}
