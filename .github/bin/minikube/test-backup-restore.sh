#!/bin/bash
#
# Copyright (c) 2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

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
  triggerNow: true
  autoconfigureRestBackupServer: true
EOF
}

createRestoreCR() {
  kubectl apply -f - <<EOF
apiVersion: org.eclipse.che/v1
kind: CheClusterRestore
metadata:
  name: eclipse-che-restore
  namespace: ${NAMESPACE}
spec:
  triggerNow: true
  copyBackupServerConfiguration: true
EOF
}

waitBackupFinished() {
  maxAttempts=25
  count=0
  while [ $count -le $maxAttempts ]; do
    statusMessage=$(kubectl get checlusterbackup eclipse-che-backup -n ${NAMESPACE} -o jsonpath='{.status.message}')
    if [[ "$statusMessage" == *'successfully finished'* ]]; then
      break
    fi
    if echo "$statusMessage" | grep -iqF error ; then
      echo "[ERROR] Filed to backup Che: $statusMessage."
      exit 1
    fi

    if [ $x -gt $maxAttempts ]; then
      echo "[ERROR] Filed to create backup: timeout."
      echo "[INFO] Backup state: ${statusMessage}"
      kubectl get pods -n ${NAMESPACE}
      exit 1
    fi

    sleep 10
    count=$((count+1))
  done
}

waitRestoreFinished() {
  maxAttempts=75
  count=0
  while [ $count -le $maxAttempts ]; do
    statusMessage=$(kubectl get checlusterrestore eclipse-che-restore -n ${NAMESPACE} -o jsonpath='{.status.message}')
    if [[ "$statusMessage" == *'successfully finished'* ]]; then
      break
    fi
    if echo "$statusMessage" | grep -iqF error ; then
      stage=$(kubectl get checlusterrestore eclipse-che-restore -n ${NAMESPACE} -o jsonpath='{.status.stage}')
      echo "[ERROR] Filed to restore Che: $stage."
      exit 1
    fi

    if [ $x -gt $maxAttempts ]; then
      echo "[ERROR] Filed to restore Che: timeout."
      stage=$(kubectl get checlusterrestore eclipse-che-restore -n ${NAMESPACE} -o jsonpath='{.status.stage}')
      echo "[INFO] Restore state: $stage"
      kubectl get pods -n ${NAMESPACE}
      exit 1
    fi

    sleep 10
    count=$((count+1))
  done
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
  startExistedWorkspace
  waitWorkspaceStart
}

prepareTemplates() {
  disableUpdateAdminPassword ${TEMPLATES}
  setIngressDomain ${TEMPLATES} "$(minikube ip).nip.io"
  setCustomOperatorImage ${TEMPLATES} ${OPERATOR_IMAGE}
}

initDefaults
initLatestTemplates
prepareTemplates
buildCheOperatorImage
copyCheOperatorImageToMinikube
runTest
