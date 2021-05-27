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

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.4-200 as builder
RUN microdnf install -y rpm gcc go-srpm-macros unzip
RUN ARCH=$(uname -m) && \
    if [ $ARCH == "s390x" ] || [ $ARCH == "ppc64le" ]; then \
        GO_VERSION=1.14-1; \
        curl -L https://rpmfind.net/linux/fedora-secondary/releases/32/Everything/${ARCH}/os/Packages/g/golang-src-${GO_VERSION}.fc32.noarch.rpm -o golang-src.rpm ; \
        curl -L https://rpmfind.net/linux/fedora-secondary/releases/32/Everything/${ARCH}/os/Packages/g/golang-bin-${GO_VERSION}.fc32.${ARCH}.rpm -o golang-bin.rpm; \
        curl -L https://rpmfind.net/linux/fedora-secondary/releases/32/Everything/${ARCH}/os/Packages/g/golang-${GO_VERSION}.fc32.${ARCH}.rpm -o golang.rpm; \
    else \
        GO_VERSION=1.14.15-3; \
        curl -L https://rpmfind.net/linux/fedora/linux/updates/32/Everything/${ARCH}/Packages/g/golang-src-${GO_VERSION}.fc32.noarch.rpm -o golang-src.rpm ; \
        curl -L https://rpmfind.net/linux/fedora/linux/updates/32/Everything/${ARCH}/Packages/g/golang-bin-${GO_VERSION}.fc32.${ARCH}.rpm -o golang-bin.rpm; \
        curl -L https://rpmfind.net/linux/fedora/linux/updates/32/Everything/${ARCH}/Packages/g/golang-${GO_VERSION}.fc32.${ARCH}.rpm -o golang.rpm; \
    fi && \
    rpm -Uhv go* && \
    go version

ARG DEV_WORKSPACE_CONTROLLER_VERSION="main"
ARG DEV_WORKSPACE_CHE_OPERATOR_VERSION="main"

USER root
ADD . /che-operator
WORKDIR /che-operator

# build operator
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    export MOCK_API=true && \
    go test -mod=vendor -v ./... && \
    GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go

# upstream, download devworkspace-operator templates for every build
# downstream, copy prefetched zip into /tmp
RUN curl -L https://api.github.com/repos/devfile/devworkspace-operator/zipball/${DEV_WORKSPACE_CONTROLLER_VERSION} > /tmp/devworkspace-operator.zip && \
    unzip /tmp/devworkspace-operator.zip */deploy/deployment/* -d /tmp && \
    mkdir -p /tmp/devworkspace-operator/templates/ && \
    mv /tmp/devfile-devworkspace-operator-*/deploy /tmp/devworkspace-operator/templates/

# upstream, download devworkspace-che-operator templates for every build
# downstream, copy prefetched zip into /tmp
RUN curl -L https://api.github.com/repos/che-incubator/devworkspace-che-operator/zipball/${DEV_WORKSPACE_CHE_OPERATOR_VERSION} > /tmp/devworkspace-che-operator.zip && \
    unzip /tmp/devworkspace-che-operator.zip */deploy/deployment/* -d /tmp && \
    mkdir -p /tmp/devworkspace-che-operator/templates/ && \
    mv /tmp/che-incubator-devworkspace-che-operator-*/deploy /tmp/devworkspace-che-operator/templates/

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.4-200

COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /che-operator/templates/keycloak-provision.sh /tmp/keycloak-provision.sh
COPY --from=builder /che-operator/templates/keycloak-update.sh /tmp/keycloak-update.sh
COPY --from=builder /che-operator/templates/oauth-provision.sh /tmp/oauth-provision.sh
COPY --from=builder /che-operator/templates/delete-identity-provider.sh /tmp/delete-identity-provider.sh
COPY --from=builder /che-operator/templates/create-github-identity-provider.sh /tmp/create-github-identity-provider.sh
COPY --from=builder /tmp/devworkspace-operator/templates/deploy /tmp/devworkspace-operator/templates
COPY --from=builder /tmp/devworkspace-che-operator/templates/deploy /tmp/devworkspace-che-operator/templates

# install httpd-tools for /usr/bin/htpasswd
RUN microdnf install -y httpd-tools && microdnf -y update && microdnf -y clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages"
CMD ["che-operator"]

# append Brew metadata here - see https://github.com/redhat-developer/codeready-workspaces-images/blob/crw-2-rhel-8/crw-jenkins/jobs/CRW_CI/crw-operator_2.x.jenkinsfile
