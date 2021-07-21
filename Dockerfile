# Copyright (c) 2018-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8/go-toolset
FROM registry.access.redhat.com/ubi8/go-toolset:1.15.13-4 as builder
ENV GOPATH=/go/
ENV RESTIC_TAG=v0.12.0
ARG DEV_WORKSPACE_CONTROLLER_VERSION="main"
USER root

WORKDIR /che-operator
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY templates/ templates/
COPY pkg/ pkg/
COPY vendor/ vendor/

# upstream, download zips for every build
# downstream, copy prefetched asset-*.zip into /tmp
RUN curl -sSLo /tmp/asset-devworkspace-operator.zip https://api.github.com/repos/devfile/devworkspace-operator/zipball/${DEV_WORKSPACE_CONTROLLER_VERSION} && \
    curl -sSLo /tmp/asset-devworkspace-che-operator.zip https://api.github.com/repos/che-incubator/devworkspace-che-operator/zipball/${DEV_WORKSPACE_CHE_OPERATOR_VERSION} && \
    curl -sSLo /tmp/asset-restic.zip https://api.github.com/repos/restic/restic/zipball/${RESTIC_TAG}

# build operator
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o che-operator main.go

RUN unzip /tmp/asset-devworkspace-operator.zip */deploy/deployment/* -d /tmp && \
    mkdir -p /tmp/devworkspace-operator/templates/ && \
    mv /tmp/devfile-devworkspace-operator-*/deploy /tmp/devworkspace-operator/templates/

# Build restic. Needed for backup / restore capabilities
RUN mkdir -p $GOPATH/restic && cd $GOPATH/restic && \
    unzip /tmp/asset-restic.zip -d /tmp && \
    mv /tmp/restic-*/* $GOPATH/restic && \
    export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    go mod vendor && \
    GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/restic/restic ./cmd/restic

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.4-205

COPY --from=builder /che-operator/che-operator /manager
COPY --from=builder /che-operator/templates/*.sh /tmp/
COPY --from=builder /tmp/devworkspace-operator/templates/deploy /tmp/devworkspace-operator/templates
COPY --from=builder /tmp/restic/restic /usr/local/bin/restic
COPY --from=builder /go/restic/LICENSE /usr/local/bin/restic-LICENSE.txt

# install httpd-tools for /usr/bin/htpasswd
RUN microdnf install -y httpd-tools && microdnf -y update && microdnf -y clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages" && \
    mkdir ~/.ssh && chmod 0766  ~/.ssh

WORKDIR /
USER 65532:65532

ENTRYPOINT ["/manager"]

# append Brew metadata here - see https://github.com/redhat-developer/codeready-workspaces-images/blob/crw-2-rhel-8/crw-jenkins/jobs/CRW_CI/crw-operator_2.x.jenkinsfile
