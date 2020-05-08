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

trap 'Catch_Finish $?' EXIT SIGINT

# Catch errors and force to delete minikube VM.
Catch_Finish() {
  rm -rf ~/.kube && yes minikube | minishift delete
}

function initialize() {
  SCRIPT=$(readlink -f "$0")
  SCRIPTPATH=$(dirname "$SCRIPT")
  
  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPTPATH");
  fi
  
  RAM_MEMORY=14000
}

function installAndStartMinishift() {
  echo "======== Start to install minishift ========"
  curl -Lo minishift.tgz https://github.com/minishift/minishift/releases/download/v1.34.2/minishift-1.34.2-linux-amd64.tgz
  tar -xvf minishift.tgz --strip-components=1
  chmod +x ./minishift
  mv ./minishift /usr/local/bin/minishift

  #Setup GitHub token for minishift
  if [ -z "$CHE_BOT_GITHUB_TOKEN" ]
  then
    echo "\$CHE_BOT_GITHUB_TOKEN is empty. Minishift start might fail with GitGub API rate limit reached."
  else
    echo "\$CHE_BOT_GITHUB_TOKEN is set, checking limits."
    GITHUB_RATE_REMAINING=$(curl -slL "https://api.github.com/rate_limit?access_token=$CHE_BOT_GITHUB_TOKEN" | jq .rate.remaining)
    if [ "$GITHUB_RATE_REMAINING" -gt 1000 ]
    then
      echo "Github rate greater than 1000. Using che-bot token for minishift startup."
      export MINISHIFT_GITHUB_API_TOKEN=$CHE_BOT_GITHUB_TOKEN
    else
      echo "Github rate is lower than 1000. *Not* using che-bot for minishift startup."
      echo "If minishift startup fails, please try again later."
    fi
  fi

  minishift version
  minishift config set memory 14GB
  minishift config set cpus 4

  echo "======== Lunch minishift ========"
  minishift start
}

function installDependencies() {
  source ${OPERATOR_REPO}/.ci/util/ci_common.sh

  installYQ
  installJQ
  install_VirtPackages
  installStartDocker
  load_jenkins_vars
}

function testUpdateOpenshift() {
    ${OPERATOR_REPO}/olm/testUpdate.sh kubernetes stable che
}

function testUpdateMinikube() {
    source ${OPERATOR_REPO}/.ci/start-minikube.sh

    ${OPERATOR_REPO}/olm/testUpdate.sh kubernetes stable che
}

function testUpdateMinishift() {
    bash <(curl -sL https://www.eclipse.org/che/chectl/) --channel=next

    cat >/tmp/che-cr-patch.yaml <<EOL
spec:
  server:
    selfSignedCert: false
    tlsSupport: false
EOL

    echo "======= Che cr patch ======="
    cat /tmp/che-cr-patch.yaml
    if chectl server:start --listr-renderer=verbose -a operator -p minishift --k8spodreadytimeout=360000 --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml; then
        echo "Started succesfully"
        oc get checluster -o yaml
    else
        echo "======== oc get events ========"
    fi

    chectl update stable
    chectl server:update --platform=minishift  --installer=operator
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
installDependencies
testUpdateMinikube
installAndStartMinishift
testUpdateMinishift
