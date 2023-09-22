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

# git ROOT directory used to mount filesystem
GIT_ROOT_DIRECTORY=$(git rev-parse --show-toplevel)

# Container image
IMAGE_NAME="eclipse/che-operator-dev"

init() {
  BLUE='\033[1;34m'
  GREEN='\033[0;32m'
  RED='\033[0;31m'
  NC='\033[0m'
  BOLD='\033[1m'
}

# Build image
build() {
  printf "%bBuilding image %b${IMAGE_NAME}${NC}..." "${BOLD}" "${BLUE}"
  if docker build -t ${IMAGE_NAME} > docker-build-log 2>&1  -<<EOF
  FROM golang:1.18-bullseye
  RUN apt update && apt install python3-pip skopeo jq rsync unzip -y && pip install yq && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    rm -rf /che-operator/bin
  RUN adduser --disabled-password --gecos "" user
  ENV GO111MODULE=on
  ENV GOPATH=/home/user/go
  ENV PATH=/home/user/go/bin:\$PATH
  USER 1000
  WORKDIR /che-operator
EOF
then
  printf "%b[OK]%b\n" "${GREEN}" "${NC}"
  rm docker-build-log
else
  printf "%bFailure%b\n" "${RED}" "${NC}"
  cat docker-build-log
  exit 1
fi
}

run() {
  printf "%bRunning%b $*\n" "${BOLD}" "${NC}"
  if docker run --rm -it -v "${GIT_ROOT_DIRECTORY}":/che-operator ${IMAGE_NAME} "$@"
  then
    printf "Script execution %b[OK]%b\n" "${GREEN}" "${NC}"
  else
    printf "%bFail to run the script%b\n" "${RED}" "${NC}"
    exit 1
  fi
}

init "$@"
build "$@"
run "$@"
