# Copyright (c) 2018-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# NOTE: using registry.access.redhat.com/rhel8/go-toolset does not work (user is requested to use registry.redhat.io)
# NOTE: using registry.redhat.io/rhel8/go-toolset requires login, which complicates automation
# NOTE: since updateBaseImages.sh does not support other registries than RHCC, update to RHEL8
# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/devtools/go-toolset-rhel7
ARG DEV_WORKSPACE_CONTROLLER_VERSION="main"
ARG DEV_WORKSPACE_CHE_OPERATOR_VERSION="main"

FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.13.15-4  as builder
ENV PATH=/opt/rh/go-toolset-1.13/root/usr/bin:${PATH} \
    GOPATH=/go/

USER root
ADD . /che-operator
WORKDIR /che-operator

# build operator
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    export MOCK_API=true && \
    go test -mod=vendor -v ./... && \
    GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go

# download devworkspace-operator templates
RUN curl -L https://api.github.com/repos/devfile/devworkspace-operator/zipball/${DEV_WORKSPACE_CONTROLLER_VERSION} > /tmp/devworkspace-operator.zip && \
    unzip /tmp/devworkspace-operator.zip */deploy/deployment/* -d /tmp

# download devworkspace-che-operator templates
RUN curl -L https://api.github.com/repos/che-incubator/devworkspace-che-operator/zipball/${DEV_WORKSPACE_CHE_OPERATOR_VERSION} > /tmp/devworkspace-che-operator.zip && \
    unzip /tmp/devworkspace-che-operator.zip */deploy/deployment/* -d /tmp


# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.3-291

COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /che-operator/templates/keycloak-provision.sh /tmp/keycloak-provision.sh
COPY --from=builder /che-operator/templates/oauth-provision.sh /tmp/oauth-provision.sh
COPY --from=builder /che-operator/templates/delete-identity-provider.sh /tmp/delete-identity-provider.sh
COPY --from=builder /che-operator/templates/create-github-identity-provider.sh /tmp/create-github-identity-provider.sh
COPY --from=builder /tmp/devfile-devworkspace-operator-*/deploy /tmp/devworkspace-operator/templates
COPY --from=builder /tmp/che-incubator-devworkspace-che-operator-*/deploy /tmp/devworkspace-che-operator/templates

# apply CVE fixes, if required
RUN microdnf update -y librepo libnghttp2 && microdnf install httpd-tools && microdnf clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages"
CMD ["che-operator"]

# append Brew metadata here (it will be appended via https://github.com/redhat-developer/codeready-workspaces-operator/blob/master/operator.Jenkinsfile)
