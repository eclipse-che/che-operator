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
    curl -L https://github.com/dhiltgen/docker-machine-kvm/releases/download/v0.10.0/docker-machine-driver-kvm-centos7 -o /usr/bin/docker-machine-driver-kvm
    chmod +x /usr/bin/docker-machine-driver-kvm
    check_libvirtd=$(systemctl is-active libvirtd)
    if [ $check_libvirtd != 'active' ]; then
        virsh net-start default
    fi
}

minishift_installation() {
  MSFT_RELEASE="1.34.2"
  printInfo "Downloading Minishift binaries"
  if [ ! -d "$OPERATOR_REPO/tmp" ]; then mkdir -p "$OPERATOR_REPO/tmp" && chmod 777 "$OPERATOR_REPO/tmp"; fi
  curl -L https://github.com/minishift/minishift/releases/download/v$MSFT_RELEASE/minishift-$MSFT_RELEASE-linux-amd64.tgz \
    -o ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar && tar -xvf ${OPERATOR_REPO}/tmp/minishift-$MSFT_RELEASE-linux-amd64.tar -C /usr/bin --strip-components=1
  printInfo "Starting a new OC cluster."
  minishift start --memory=4096 && eval $(minishift oc-env)
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
    echo "exclude=mirror1.ci.centos.org" >> /etc/yum/pluginconf.d/fastestmirror.conf
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
