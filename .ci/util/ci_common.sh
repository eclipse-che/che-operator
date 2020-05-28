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

printInfo() {
  set +x
  echo ""
  echo "[=============== [INFO] $1 ===============]"
}

printWarn() {
  set +x
  echo ""
  echo "[=============== [WARN] $1 ===============]"
}

printError() {
  set +x
  echo ""
  echo "[=============== [ERROR] $1 ===============]"
}

installStartDocker() {
  if [ -x "$(command -v docker)" ]; then
    printWarn "Docker already installed"
  else
    printInfo "Installing docker..."
    yum install --assumeyes -d1 yum-utils device-mapper-persistent-data lvm2
    yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    
    printInfo "Starting docker service..."
    yum install --assumeyes -d1 docker-ce
    systemctl start docker
    docker version
  fi
}

install_VirtPackages() {
  printInfo 'Installing required virtualization packages installed'
  sudo yum -y install libvirt qemu-kvm
}

start_libvirt() {
  systemctl start libvirtd
}

setup_kvm_machine_driver() {
    printInfo "Installing docker machine kvm drivers"
    curl -L https://github.com/dhiltgen/docker-machine-kvm/releases/download/v0.10.0/docker-machine-driver-kvm-centos7 -o /usr/local/bin/docker-machine-driver-kvm
    chmod +x /usr/local/bin/docker-machine-driver-kvm
    check_libvirtd=$(systemctl is-active libvirtd)
    if [ $check_libvirtd != 'active' ]; then
        virsh net-start default
    fi
}

github_token_set() {
  #Setup GitHub token for minishift
  if [ -z "$CHE_BOT_GITHUB_TOKEN" ]
  then
    printWarn "\$CHE_BOT_GITHUB_TOKEN is empty. Minishift start might fail with GitGub API rate limit reached."
  else
    printInfo "\$CHE_BOT_GITHUB_TOKEN is set, checking limits."
    GITHUB_RATE_REMAINING=$(curl -slL "https://api.github.com/rate_limit?access_token=$CHE_BOT_GITHUB_TOKEN" | jq .rate.remaining)
    if [ "$GITHUB_RATE_REMAINING" -gt 1000 ]
    then
      printInfo "Github rate greater than 1000. Using che-bot token for minishift startup."
      export MINISHIFT_GITHUB_API_TOKEN=$CHE_BOT_GITHUB_TOKEN
    else
      printInfo "Github rate is lower than 1000. *Not* using che-bot for minishift startup."
      printInfo "If minishift startup fails, please try again later."
    fi
  fi
}

minishift_installation() {
  MSFT_RELEASE="1.34.2"
  printInfo "Downloading Minishift binaries"
  if [ ! -d "$OPERATOR_REPO/tmp" ]; then mkdir -p "$OPERATOR_REPO/tmp" && chmod 777 "$OPERATOR_REPO/tmp"; fi
  curl -L https://github.com/minishift/minishift/releases/download/v$MSFT_RELEASE/minishift-$MSFT_RELEASE-linux-amd64.tgz \
    -o ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar && tar -xvf ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar -C /usr/bin --strip-components=1
  
  printInfo "Setting github token and start a new minishift VM."
  github_token_set
  minishift start --memory=8192 && eval $(minishift oc-env)
  oc login -u system:admin
  oc adm policy add-cluster-role-to-user cluster-admin developer && oc login -u developer -p developer
  printInfo "Successfully started OCv3.X on minishift machine"
}

generate_self_signed_certs() {
  IP_ADDRESS="172.17.0.1"
  openssl req -x509 \
              -newkey rsa:4096 \
              -keyout key.pem \
              -out cert.pem \
              -days 365 \
              -subj "/CN=*.${IP_ADDRESS}.nip.io" \
              -nodes && cat cert.pem key.pem > ca.crt    
}

minikube_installation() {
  start_libvirt
  curl -Lo minikube https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 \
  && chmod +x minikube
  sudo install minikube /usr/local/bin/ && rm -rf minikube
}

installEpelRelease() {
  if yum repolist | grep epel; then
    printWarn "Epel already installed, skipping instalation."
  else
    #excluding mirror1.ci.centos.org 
    printInfo "Installing epel..."
    yum install -d1 --assumeyes epel-release
    yum update --assumeyes -d1
  fi
}

installYQ() {
  printInfo "Installing yq portable command-line YAML processor..."
  sudo yum install --assumeyes -d1 python3-pip
  pip3 install --upgrade setuptools
  pip3 install yq
}

installJQ() {
  installEpelRelease
  yum install --assumeyes -d1 jq
}

installChectl() {
  printInfo "Installing chectl"
  bash <(curl -sL https://www.eclipse.org/che/chectl/) --channel=next

}

getCheAcessToken() {
  KEYCLOAK_HOSTNAME=keycloak-che.$(minikube ip).nip.io
  TOKEN_ENDPOINT="https://${KEYCLOAK_HOSTNAME}/auth/realms/che/protocol/openid-connect/token"
  export CHE_ACCESS_TOKEN=$(curl --data "grant_type=password&client_id=che-public&username=admin&password=admin" -k ${TOKEN_ENDPOINT} | jq -r .access_token)
}

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

load_jenkins_vars() {
    set +x
    eval "$(./env-toolkit load -f jenkins-env.json \
                              CHE_BOT_GITHUB_TOKEN)"
}
