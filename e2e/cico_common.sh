#!/bin/bash
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

installStartDocker() {
  if [ -x "$(command -v docker)" ]; then
    echo "[INFO] Docker already installed"
  else
    echo "[INFO] Installing docker..."
    yum install --assumeyes -d1 yum-utils device-mapper-persistent-data lvm2
    yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    yum install --assumeyes -d1 docker-ce
    systemctl start docker
    docker version
  fi
}

install_required_packages() {
    # Install EPEL repo
    yum -y install epel-release
    # Get all the deps in
    yum -y install libvirt qemu-kvm
 
  echo '[INFO]CICO: Required virtualization packages installed'
}

start_libvirt() {
  systemctl start libvirtd
}

setup_kvm_machine_driver() {
    echo "[INFO] Installing docker machine kvm drivers..."
    curl -L https://github.com/dhiltgen/docker-machine-kvm/releases/download/v0.10.0/docker-machine-driver-kvm-centos7 -o /usr/bin/docker-machine-driver-kvm
    chmod +x /usr/bin/docker-machine-driver-kvm

    check_libvirtd=$(systemctl is-active libvirtd)
    
    if [ $check_libvirtd != 'active' ]; then
        virsh net-start default
    fi
}
