#!/bin/bash
#
# Copyright (c) 2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

command -v delv >/dev/null 2>&1 || { echo "delv is not installed. Aborting."; exit 1; }
command -v operator-sdk >/dev/null 2>&1 || { echo "operator-sdk is not installed. Aborting."; exit 1; }

ECLIPSE_CHE_NAMESPACE="eclipse-che"
ECLIPSE_CHE_CR="./deploy/crds/org_v1_che_cr.yaml"
ECLIPSE_CHE_CRD="./deploy/crds/org_v1_che_crd.yaml"
ECLIPSE_CHE_BACKUP_CRD="./deploy/crds/org.eclipse.che_checlusterbackups_crd.yaml"
ECLIPSE_CHE_RESTORE_CRD="./deploy/crds/org.eclipse.che_checlusterrestores_crd.yaml"
DEV_WORKSPACE_CONTROLLER_VERSION="main"
DEV_WORKSPACE_CHE_OPERATOR_VERSION="main"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Catch_Finish is executed after finish script.
catchFinish() {
  if [ -n "${OPERATOR_SDK_PID}" ]; then
    # Gracefull SIG_TERM process
    kill -15 "${OPERATOR_SDK_PID}"
    echo "Debug completed."
  fi
}

usage () {
	echo "Usage:   $0 [-n ECLIPSE_CHE_NAMESPACE] [-cr ECLIPSE_CHE_CR] "
}

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-n')  ECLIPSE_CHE_NAMESPACE="$2"; shift 1;;
    '-cr') ECLIPSE_CHE_CR="$2"; shift 1;;
	'--help'|'-h') usage; exit;;
  esac
  shift 1
done

prepareTemplates() {
  cp templates/keycloak-provision.sh /tmp/keycloak-provision.sh
  cp templates/delete-identity-provider.sh /tmp/delete-identity-provider.sh
  cp templates/create-github-identity-provider.sh /tmp/create-github-identity-provider.sh
  cp templates/oauth-provision.sh /tmp/oauth-provision.sh
  cp templates/keycloak-update.sh /tmp/keycloak-update.sh

  # Download Dev Workspace operator templates
  echo "[INFO] Downloading Dev Workspace operator templates ..."
  rm -f /tmp/devworkspace-operator.zip
  rm -rf /tmp/devfile-devworkspace-operator-*
  rm -rf /tmp/devworkspace-operator/
  mkdir -p /tmp/devworkspace-operator/templates

  curl -sL https://api.github.com/repos/devfile/devworkspace-operator/zipball/${DEV_WORKSPACE_CONTROLLER_VERSION} > /tmp/devworkspace-operator.zip

  unzip /tmp/devworkspace-operator.zip '*/deploy/deployment/*' -d /tmp
  cp -r /tmp/devfile-devworkspace-operator*/deploy/* /tmp/devworkspace-operator/templates
  echo "[INFO] Downloading Dev Workspace operator templates completed."

  # Download Dev Workspace Che operator templates
  echo "[INFO] Downloading Dev Workspace Che operator templates ..."
  rm -f /tmp/devworkspace-che-operator.zip
  rm -rf /tmp/che-incubator-devworkspace-che-operator-*
  rm -rf /tmp/devworkspace-che-operator/
  mkdir -p /tmp/devworkspace-che-operator/templates

  curl -sL https://api.github.com/repos/che-incubator/devworkspace-che-operator/zipball/${DEV_WORKSPACE_CHE_OPERATOR_VERSION} > /tmp/devworkspace-che-operator.zip

  unzip /tmp/devworkspace-che-operator.zip '*/deploy/deployment/*' -d /tmp
  cp -r /tmp/che-incubator-devworkspace-che-operator*/deploy/* /tmp/devworkspace-che-operator/templates
  echo "[INFO] Downloading Dev Workspace Che operator templates completed."
}

createNamespace() {
  set +e
  kubectl create namespace $ECLIPSE_CHE_NAMESPACE
  set -e
}

applyCRandCRD() {
  kubectl apply -f ${ECLIPSE_CHE_CRD}
  kubectl apply -f ${ECLIPSE_CHE_BACKUP_CRD}
  kubectl apply -f ${ECLIPSE_CHE_RESTORE_CRD}
  kubectl apply -f ${ECLIPSE_CHE_CR} -n $ECLIPSE_CHE_NAMESPACE
}

runDebug() {
  ENV_FILE=/tmp/che-operator-debug.env
  rm -rf "${ENV_FILE}"
  touch "${ENV_FILE}"
  CLUSTER_API_URL=$(oc whoami --show-server=true) || true
  if [ -n "${CLUSTER_API_URL}" ]; then
      echo "CLUSTER_API_URL='${CLUSTER_API_URL}'" >> "${ENV_FILE}"
      echo "[INFO] Set up cluster api url: ${CLUSTER_API_URL}"
  fi
  echo "WATCH_NAMESPACE='${ECLIPSE_CHE_NAMESPACE}'" >> ${ENV_FILE}

  echo "[WARN] Make sure that your CR contains valid ingress domain!"

  operator-sdk run --local --watch-namespace ${ECLIPSE_CHE_NAMESPACE} --enable-delve &
  OPERATOR_SDK_PID=$!

  wait ${OPERATOR_SDK_PID}
}

prepareTemplates
createNamespace
applyCRandCRD
runDebug
