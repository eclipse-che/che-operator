#!/bin/bash
#
# Copyright (c) 2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -ex

#Stop execution on any error
trap "catchFinish" EXIT SIGINT

# Catch_Finish is executed after finish script.
catchFinish() {
  result=$?
  if [ "$result" != "0" ]; then
    printInfo "Failed on running tests. Please check logs or contact QE team (e-mail:codereadyqe-workspaces-qe@redhat.com, Slack: #che-qe-internal, Eclipse mattermost: 'Eclipse Che QE'"
    printInfo "Logs should be availabe on http://artifacts.ci.centos.org/devtools/che/che-eclipse-minikube-updates/${ghprbPullId}/"
    exit 1
    getCheClusterLogs
    archiveArtifacts "che-eclipse-minikube-updates"
  fi
  minikube delete && yes | kubeadm reset
  rm -rf ~/.kube ~/.minikube
  exit $result
}

init() {
  SCRIPT=$(readlink -f "$0")
  SCRIPT_DIR=$(dirname "$SCRIPT")

  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

  RAM_MEMORY=8192
  PLATFORM="kubernetes"
  NAMESPACE="che"
  CHANNEL="stable"
}

installDependencies() {
  installYQ
  installJQ
  install_VirtPackages
  installStartDocker
  source ${OPERATOR_REPO}/.ci/start-minikube.sh
}

testUpdates() {
  "${OPERATOR_REPO}"/olm/testUpdate.sh ${PLATFORM} ${CHANNEL} ${NAMESPACE}
  printInfo "Successfully verified updates on kubernetes platform."
  installChectl
  getCheAcessToken
  chectl workspace:create --start --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml
  chectl workspace:list
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
installDependencies
testUpdates

getCheClusterLogs() {
  mkdir -p /root/payload/report/che-logs
  cd /root/payload/report/che-logs
  for POD in $(kubectl get pods -o name -n ${NAMESPACE}); do
    for CONTAINER in $(kubectl get -n ${NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
      echo ""
      printInfo "Getting logs from $POD"
      echo ""
      kubectl logs ${POD} -c ${CONTAINER} -n ${NAMESPACE} |tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
    done
  done
  printInfo "kubectl get events"
  kubectl get events -n ${NAMESPACE}| tee get_events.log
  printInfo "kubectl get all"
  kubectl get all | tee get_all.log
}

## $1 = name of subdirectory into which the artifacts will be archived. Commonly it's job name.
archiveArtifacts() {
  JOB_NAME=$1
  DATE=$(date +"%m-%d-%Y-%H-%M")
  echo "Archiving artifacts from ${DATE} for ${JOB_NAME}/${BUILD_NUMBER}"
  cd /root/payload
  ls -la ./artifacts.key
  chmod 600 ./artifacts.key
  chown $(whoami) ./artifacts.key
  mkdir -p ./che/${JOB_NAME}/${BUILD_NUMBER}
  cp -R ./report ./che/${JOB_NAME}/${BUILD_NUMBER}/ | true
  rsync --password-file=./artifacts.key -Hva --partial --relative ./che/${JOB_NAME}/${BUILD_NUMBER} devtools@artifacts.ci.centos.org::devtools/
}
