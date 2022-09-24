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

# https://registry.access.redhat.com/ubi8/go-toolset
FROM registry.access.redhat.com/ubi8/go-toolset:1.17.12-7 as builder
ENV GOPATH=/go/
ARG DEV_HEADER_REWRITE_TRAEFIK_PLUGIN="main"
ARG SKIP_TESTS="false"
USER root

# upstream, download zips for every build
# downstream, copy prefetched asset-*.zip into /tmp
RUN curl -sSLo /tmp/asset-header-rewrite-traefik-plugin.zip https://api.github.com/repos/che-incubator/header-rewrite-traefik-plugin/zipball/${DEV_HEADER_REWRITE_TRAEFIK_PLUGIN}

WORKDIR /che-operator

RUN unzip /tmp/asset-header-rewrite-traefik-plugin.zip -d /tmp && \
    mkdir -p /tmp/header-rewrite-traefik-plugin && \
    mv /tmp/*-header-rewrite-traefik-plugin-*/headerRewrite.go /tmp/*-header-rewrite-traefik-plugin-*/.traefik.yml /tmp/header-rewrite-traefik-plugin

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY main.go main.go
COPY vendor/ vendor/
COPY mocks/ mocks/
COPY api/ api/
COPY config/ config/
COPY controllers/ controllers/
COPY pkg/ pkg/

# build operator
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    if [[ ${SKIP_TESTS} == "false" ]]; then export MOCK_API=true && go test -mod=vendor -v ./...; fi && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} GO111MODULE=on go build -mod=vendor -a -o che-operator main.go

# https://registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.6-902.1661794353

COPY --from=builder /tmp/header-rewrite-traefik-plugin /tmp/header-rewrite-traefik-plugin
COPY --from=builder /che-operator/che-operator /manager

ENTRYPOINT ["/manager"]

# append Brew metadata here
