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
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.11.5-3 as builder
ENV PATH=/opt/rh/go-toolset-1.11/root/usr/bin:$PATH \
    GOPATH=/go/

USER root
ADD . /go/src/github.com/eclipse/che-operator
RUN cd /go/src/github.com/eclipse/che-operator && export MOCK_API=true && go test -v ./... && \
    OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/che-operator/che-operator \
    /go/src/github.com/eclipse/che-operator/cmd/manager/main.go && cd ..

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/rhel7
FROM registry.access.redhat.com/rhel7:7.6-202

ENV SUMMARY="Red Hat CodeReady Workspaces Operator container" \
    DESCRIPTION="Red Hat CodeReady Workspaces Operator container" \
    PRODNAME="codeready-workspaces" \
    COMPNAME="operator-container"

LABEL summary="$SUMMARY" \
      description="$DESCRIPTION" \
      io.k8s.description="$DESCRIPTION" \
      io.k8s.display-name="Red Hat CodeReady Workspaces for OpenShift - Operator" \
      io.openshift.tags="$PRODNAME,$COMPNAME" \
      com.redhat.component="$PRODNAME-$COMPNAME" \
      name="$PRODNAME/$COMPNAME" \
      version="1.1" \
      license="EPLv2" \
      maintainer="Nick Boldt <nboldt@redhat.com>" \
      io.openshift.expose-services="" \
      usage=""

COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /go/src/github.com/eclipse/che-operator/deploy/keycloak_provision /tmp/keycloak_provision
RUN yum list installed && echo "End Of Installed Packages"
CMD ["che-operator"]