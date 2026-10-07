# Copyright (c) 2019-2026 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

FROM registry.access.redhat.com/ubi8:8.10-1304.1751400627 as builder

USER root

ENV GOPATH=/go
ENV GOROOT=/usr/local/go
ENV CGO_ENABLED=1
ENV GO_VERSION=1.26.5
ENV PATH=$PATH:$GOROOT/bin:/usr/local/bin

ARG SKIP_TESTS="false"

RUN dnf install -y unzip gcc make curl && dnf clean all

# go v1.26.5 installation
RUN ARCH="$(uname -m)" && \
    if [ "${ARCH}" = "x86_64" ]; then \
        ARCH="amd64"; \
    elif [ "${ARCH}" = "aarch64" ]; then \
        ARCH="arm64"; \
    fi && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz && \
    go version

WORKDIR /che-operator

COPY go.mod go.mod
COPY go.sum go.sum
COPY Makefile Makefile
COPY cmd/ cmd/
COPY vendor/ vendor/
COPY api/ api/
COPY config/ config/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY editors-definitions /tmp/editors-definitions
COPY header-rewrite-traefik-plugin /tmp/header-rewrite-traefik-plugin

RUN if [ "${SKIP_TESTS}" = "false" ]; then \
      make test; \
    fi

# build operator
# to test FIPS compliance, run https://github.com/openshift/check-payload#scan-a-container-or-operator-image against a built image
RUN ARCH="$(uname -m)" && \
    if [ "${ARCH}" = "x86_64" ]; then \
        ARCH="amd64"; \
    elif [ "${ARCH}" = "aarch64" ]; then \
        ARCH="arm64"; \
    fi && \
    GOOS=linux GOARCH="${ARCH}" GO111MODULE=on \
    go build -mod=vendor -a -o che-operator cmd/main.go

FROM registry.access.redhat.com/ubi8-minimal:8.10-1295.1749680713

COPY --from=builder /tmp/header-rewrite-traefik-plugin /tmp/header-rewrite-traefik-plugin
COPY --from=builder /tmp/editors-definitions /tmp/editors-definitions
COPY --from=builder /che-operator/che-operator /manager

USER 1001
ENTRYPOINT ["/manager"]
