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
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.13.4-18  as builder
ENV PATH=/opt/rh/go-toolset-1.13/root/usr/bin:${PATH} \
    GOPATH=/go/

USER root
ADD . /che-operator
WORKDIR /che-operator

RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    export MOCK_API=true && go test -mod=vendor -v ./... && \
    GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.3-201

COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /che-operator/templates/keycloak_provision /tmp/keycloak_provision
COPY --from=builder /che-operator/templates/oauth_provision /tmp/oauth_provision
# apply CVE fixes, if required
RUN microdnf update -y librepo libnghttp2 && microdnf clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages"
CMD ["che-operator"]

# append Brew metadata here (it will be appended via https://github.com/redhat-developer/codeready-workspaces-operator/blob/master/operator.Jenkinsfile)
