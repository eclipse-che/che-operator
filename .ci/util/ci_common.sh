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

set -e

# Get the access token from keycloak in openshift platforms and kubernetes
function getCheAcessToken() {
  if [[ ${PLATFORM} == "openshift" ]]
  then
    export CHE_API_ENDPOINT=$(oc get route -n ${NAMESPACE} che --template={{.spec.host}})/api

    KEYCLOAK_HOSTNAME=$(oc get route -n ${NAMESPACE} keycloak --template={{.spec.host}})
    TOKEN_ENDPOINT="https://${KEYCLOAK_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
    export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
  else
    export CHE_API_ENDPOINT=che-che.$(minikube ip).nip.io/api

    KEYCLOAK_HOSTNAME=keycloak-che.$(minikube ip).nip.io
    TOKEN_ENDPOINT="https://${KEYCLOAK_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
    export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
  fi
}

# Utility to wait for a workspace to be started after workspace:create.
function waitWorkspaceStart() {
  set +e
  export x=0
  while [ $x -le 180 ]
  do
    getCheAcessToken

    chectl workspace:list --chenamespace=${NAMESPACE}
    workspaceList=$(chectl workspace:list --chenamespace=${NAMESPACE})
    workspaceStatus=$(echo "$workspaceList" | grep RUNNING | awk '{ print $4} ')
    echo -e ""

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

# Create cheCluster object in Openshift ci with desired values
function applyCRCheCluster() {
  echo "Creating Custom Resource"
  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${CSV_FILE}")
  CR=$(echo "$CRs" | yq -r ".[0]")
  if [ "${PLATFORM}" == "kubernetes" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi
  if [ "${PLATFORM}" == "openshift" ] && [ "${OAUTH}" == "false" ]; then
    CR=$(echo "$CR" | yq -r ".spec.auth.openShiftoAuth = false")
  fi

  echo "$CR" | oc apply -n "${NAMESPACE}" -f -
}

# Wait for CheCluster object to be ready
function waitCheServerDeploy() {
  echo "[INFO] Waiting for Che server to be deployed"
  set +e

  i=0
  while [[ $i -le 480 ]]
  do
    status=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o jsonpath={.status.cheClusterRunning})
    echo -e ""
    echo -e "[INFO] Che deployment status:"
    oc get pods -n "${NAMESPACE}"
    if [ "${status:-UNAVAILABLE}" == "Available" ]
    then
      break
    fi
    sleep 10
    ((i++))
  done

  if [ $i -gt 480 ]
  then
    echo "[ERROR] Che server did't start after 8 minutes"
    exit 1
  fi
}

# Utility to get all logs from che
function getCheClusterLogs() {
  mkdir -p /tmp/artifacts-che
  cd /tmp/artifacts-che

  for POD in $(kubectl get pods -o name -n ${NAMESPACE}); do
    for CONTAINER in $(kubectl get -n ${NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
      echo ""
      echo "[INFO] Getting logs from $POD"
      echo ""
      kubectl logs ${POD} -c ${CONTAINER} -n ${NAMESPACE} |tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
    done
  done
  echo "[INFO] Get events"
  kubectl get events -n ${NAMESPACE}| tee get_events.log
  kubectl get all | tee get_all.log
}
