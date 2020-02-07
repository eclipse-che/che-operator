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
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.12.12-4  as builder
ENV PATH=/opt/rh/go-toolset-1.12/root/usr/bin:$PATH \
    GOPATH=/go/

USER root
ADD . /go/src/github.com/eclipse/che-operator

# do no break RUN lines when building with UBI base images. https://projects.engineering.redhat.com/browse/OSBS-7398 & OSBS-7399
RUN cd /go/src/github.com/eclipse/che-operator && export MOCK_API=true && go test -v ./... && OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/che-operator/che-operator /go/src/github.com/eclipse/che-operator/cmd/manager/main.go && cd ..

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.1-328

COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /go/src/github.com/eclipse/che-operator/templates/keycloak_provision /tmp/keycloak_provision
COPY --from=builder /go/src/github.com/eclipse/che-operator/templates/oauth_provision /tmp/oauth_provision
# apply CVE fixes, if required
RUN microdnf update -y libnghttp2 && microdnf clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages"
CMD ["che-operator"]

# append Brew metadata here (it will be appended via https://github.com/redhat-developer/codeready-workspaces-operator/blob/master/operator.Jenkinsfile)
