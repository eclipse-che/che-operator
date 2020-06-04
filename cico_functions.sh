#!/bin/bash
#
# Copyright (c) 2012-2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

# Output command before executing
set -x

# Exit on error
set -e

# Source environment variables of the jenkins slave
# that might interest this worker.
function load_jenkins_vars() {
  if [ -e "jenkins-env.json" ]; then
    eval "$(./env-toolkit load -f jenkins-env.json \
            DEVSHIFT_TAG_LEN \
            QUAY_ECLIPSE_CHE_USERNAME \
            QUAY_ECLIPSE_CHE_PASSWORD \
            JENKINS_URL \
            GIT_BRANCH \
            GIT_COMMIT \
            BUILD_NUMBER \
            ghprbSourceBranch \
            ghprbActualCommit \
            BUILD_URL \
            ghprbPullId)"
  fi
}

function check_version() {
  local query=$1
  local target=$2
  echo "$target" "$query" | tr ' ' '\n' | sort -V | head -n1 2> /dev/null
}

function check_buildx_support() {
  docker_version="$(docker --version | cut -d' ' -f3 | tr -cd '0-9.')"
  if [[ $(check_version "$docker_version" "19.03") != 19.03 ]]; then
    echo "CICO: Docker $docker_version greater than or equal to 19.03 is required."
    exit 1
  else
         # Kernel
         kernel_version="$(uname -r)"
         if [[ $(check_version "$kernel_version" "4.8") != "4.8" ]]; then
                 echo "Kernel $kernel_version too old - need >= 4.8." \
                         " Install a newer kernel."
                 exit 1
         else
                 echo "kernel $kernel_version has binfmt_misc fix-binary (F) support."
         fi
  fi
}

function install_deps() {
  # We need to disable selinux for now, XXX
  /usr/sbin/setenforce 0  || true

  # Get all the deps in
  yum install -y yum-utils device-mapper-persistent-data lvm2
  yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
  yum install -y docker-ce \
    git

  service docker start

  #set buildx env
  export DOCKER_BUILD_KIT=1
  export DOCKER_CLI_EXPERIMENTAL=enabled  

  #Enable qemu and binfmt support
  docker run --rm --privileged docker/binfmt:66f9012c56a8316f9244ffd7622d7c21c1f6f28d
  docker run --rm --privileged multiarch/qemu-user-static:4.2.0-7 --reset -p yes

  echo 'CICO: Dependencies installed'
}

function set_nightly_tag() {
  # Let's set the tag as nightly
  export TAG="nightly"
}

function build_and_push() {
  REGISTRY="quay.io"
  ORGANIZATION="eclipse"
  IMAGE="che-operator"
  QUAY_USERNAME=${QUAY_ECLIPSE_CHE_USERNAME}
  QUAY_PASSWORD=${QUAY_ECLIPSE_CHE_PASSWORD}

  if [ -n "${QUAY_USERNAME}" ] && [ -n "${QUAY_PASSWORD}" ]; then
    docker login -u "${QUAY_USERNAME}" -p "${QUAY_PASSWORD}" "${REGISTRY}"
  else
    echo "Could not login, missing credentials for pushing to the '${ORGANIZATION}' organization"
  fi

  # Let's build and push images to 'quay.io'
  # Create a new builder instance using buildx
  docker buildx create --use --name builder
  docker buildx inspect --bootstrap

  docker buildx build --platform linux/amd64,linux/s390x -t ${REGISTRY}/${ORGANIZATION}/${IMAGE}:${TAG} --push --progress plain --no-cache .
  
  echo "CICO: '${TAG}' version of image pushed to '${REGISTRY}/${ORGANIZATION}' organization"
}
