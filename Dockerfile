# Copyright (c) 2018-2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/devtools/go-toolset-rhel7
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.11.5-3.1553822355 as builder
ENV PATH=/opt/rh/go-toolset-1.11/root/usr/bin:$PATH \
    GOPATH=/go/

USER root
ADD . /go/src/github.com/eclipse/che-operator

# do no break RUN lines when building with UBI base images. https://projects.engineering.redhat.com/browse/OSBS-7398 & OSBS-7399
RUN cd /go/src/github.com/eclipse/che-operator && export MOCK_API=true && go test -v ./... && OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/che-operator/che-operator /go/src/github.com/eclipse/che-operator/cmd/manager/main.go && cd ..

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi7/ubi
# don't use FROM ubi7/ubi
FROM registry.access.redhat.com/ubi7:7.6-123

ENV SUMMARY="Red Hat CodeReady Workspaces Operator container" \
    DESCRIPTION="Red Hat CodeReady Workspaces Operator container" \
    PRODNAME="codeready-workspaces" \
    COMPNAME="operator-rhel8"

LABEL summary="$SUMMARY" \
      description="$DESCRIPTION" \
      io.k8s.description="$DESCRIPTION" \
      io.k8s.display-name="$DESCRIPTION" \
      io.openshift.tags="$PRODNAME,$COMPNAME" \
      com.redhat.component="$PRODNAME-$COMPNAME" \
      name="$PRODNAME/$COMPNAME" \
      version="1.2" \
      license="EPLv2" \
      maintainer="Nick Boldt <nboldt@redhat.com>" \
      io.openshift.expose-services="" \
      usage=""

COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /go/src/github.com/eclipse/che-operator/deploy/keycloak_provision /tmp/keycloak_provision
# CVE fix for RHSA-2019:0679-02 https://pipeline.engineering.redhat.com/freshmakerevent/8717
# CVE-2019-9636 errata 40636 - update python and python-libs to 2.7.5-77.el7_6
# RUN yum update -y libssh2 python-libs python
RUN yum clean all && rm -rf /var/cache/yum && rpm -qa | sort -V && echo "End Of Installed Packages"
CMD ["che-operator"]
