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

# Get absolute path for root repo directory from github actions context: https://docs.github.com/en/free-pro-team@latest/actions/reference/context-and-expression-syntax-for-github-actions
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
if [ -z "${OPERATOR_REPO}" ]; then
  SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$SCRIPT")")")")
fi
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}/olm/olm.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

createBackupCR() {
  kubectl apply -f - <<EOF
apiVersion: org.eclipse.che/v1
kind: CheClusterBackup
metadata:
  name: eclipse-che-backup
  namespace: ${NAMESPACE}
spec:
  useInternalBackupServer: true
EOF
}

createRestoreCR() {
  kubectl apply -f - <<EOF
apiVersion: org.eclipse.che/v1
kind: CheClusterRestore
metadata:
  name: eclipse-che-restore
  namespace: ${NAMESPACE}
spec: {}
EOF
}

waitBackupFinished() {
  maxAttempts=25
  count=0
  while [ $count -le $maxAttempts ]; do
    state=$(kubectl get checlusterbackup eclipse-che-backup -n ${NAMESPACE} -o jsonpath='{.status.state}')
    if [[ "$state" == 'Succeeded' ]]; then
      break
    fi
    if [[ "$state" == 'Failed' ]]; then
      status=$(kubectl get checlusterbackup eclipse-che-backup -n ${NAMESPACE} -o jsonpath='{.status}')
      echo "[ERROR] Filed to backup Che: $status."
      exit 1
    fi

    sleep 10
    count=$((count+1))
  done

  if [ $count -gt $maxAttempts ]; then
    echo "[ERROR] Filed to create backup: timeout."
    status=$(kubectl get checlusterbackup eclipse-che-backup -n ${NAMESPACE} -o jsonpath='{.status}')
    echo "[INFO] Backup status: ${status}"
    kubectl get pods -n ${NAMESPACE}
    exit 1
  fi
}

waitRestoreFinished() {
  maxAttempts=130
  count=0
  while [ $count -le $maxAttempts ]; do
    state=$(kubectl get checlusterrestore eclipse-che-restore -n ${NAMESPACE} -o jsonpath='{.status.state}')
    if [[ "$state" == 'Succeeded' ]]; then
      break
    fi
    if [[ "$state" == 'Failed' ]]; then
      status=$(kubectl get checlusterrestore eclipse-che-restore -n ${NAMESPACE} -o jsonpath='{.status}')
      echo "[ERROR] Filed to restore Che: $status."
      exit 1
    fi

    sleep 10
    count=$((count+1))
  done

  if [ $count -gt $maxAttempts ]; then
    echo "[ERROR] Filed to restore Che: timeout."
    status=$(kubectl get checlusterrestore eclipse-che-restore -n ${NAMESPACE} -o jsonpath='{.status}')
    echo "[INFO] Restore status: $status"
    kubectl get pods -n ${NAMESPACE}
    exit 1
  fi
}

runTest() {
  deployEclipseCheWithTemplates "operator" "minikube" ${OPERATOR_IMAGE} ${TEMPLATES}
  createWorkspace
  startExistedWorkspace
  waitWorkspaceStart

  createBackupCR
  waitBackupFinished

  stopExistedWorkspace
  waitExistedWorkspaceStop
  deleteExistedWorkspace

  createRestoreCR
  waitRestoreFinished
  # Wait some time to let Keycloak finish its initialization after restoring of the database
  sleep 60
  startExistedWorkspace
  waitWorkspaceStart
}

patchTemplates() {
  disableUpdateAdminPassword ${TEMPLATES}
  setIngressDomain ${TEMPLATES} "$(minikube ip).nip.io"
  setCustomOperatorImage ${TEMPLATES} ${OPERATOR_IMAGE}
}

initDefaults
initLatestTemplates
patchTemplates
buildCheOperatorImage
copyCheOperatorImageToMinikube
runTest
