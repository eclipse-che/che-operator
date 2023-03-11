# Copyright (c) 2019-2023 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# https://registry.access.redhat.com/rhel8/go-toolset
FROM rhel8/go-toolset:1.18.9-13 as builder
ENV GOPATH=/go/
ARG SKIP_TESTS="false"
USER root

# cachito:gomod step 1: copy cachito sources where we can use them; source env vars; set working dir
COPY $REMOTE_SOURCES $REMOTE_SOURCES_DIR
RUN source $REMOTE_SOURCES_DIR/devspaces-images-operator/cachito.env
WORKDIR $REMOTE_SOURCES_DIR/devspaces-images-operator/app/devspaces-operator

RUN mkdir -p /tmp/devworkspace-operator/templates/ && \
    mv $REMOTE_SOURCES_DIR/DEV_WORKSPACE_CONTROLLER/app/deploy/deployment/* /tmp/devworkspace-operator/templates/

RUN mkdir -p /tmp/header-rewrite-traefik-plugin && \
    mv $REMOTE_SOURCES_DIR/DEV_HEADER_REWRITE_TRAEFIK_PLUGIN/app/headerRewrite.go /tmp/header-rewrite-traefik-plugin && \
    mv $REMOTE_SOURCES_DIR/DEV_HEADER_REWRITE_TRAEFIK_PLUGIN/app/.traefik.yml /tmp/header-rewrite-traefik-plugin

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
FROM ubi8-minimal:8.7-1085

COPY --from=builder /tmp/devworkspace-operator/templates /tmp/devworkspace-operator/templates
COPY --from=builder /tmp/header-rewrite-traefik-plugin /tmp/header-rewrite-traefik-plugin
COPY --from=builder $REMOTE_SOURCES_DIR/devspaces-images-operator/app/devspaces-operator/che-operator /manager

ENTRYPOINT ["/manager"]

# append Brew metadata here
ENV SUMMARY="Red Hat OpenShift Dev Spaces operator container" \
    DESCRIPTION="Red Hat OpenShift Dev Spaces operator container" \
    PRODNAME="devspaces" \
    COMPNAME="operator"
LABEL com.redhat.delivery.appregistry="false" \
      summary="$SUMMARY" \
      description="$DESCRIPTION" \
      io.k8s.description="$DESCRIPTION" \
      io.k8s.display-name="$DESCRIPTION" \
      io.openshift.tags="$PRODNAME,$COMPNAME" \
      com.redhat.component="$PRODNAME-rhel8-$COMPNAME-container" \
      name="$PRODNAME/$COMPNAME" \
      version="3.6" \
      license="EPLv2" \
      maintainer="Anatolii Bazko <abazko@redhat.com>, Nick Boldt <nboldt@redhat.com>, Dmytro Nochevnov <dnochevn@redhat.com>" \
      io.openshift.expose-services="" \
      usage=""
