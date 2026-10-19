#
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

FROM registry.ci.openshift.org/openshift/release:rhel-9-release-golang-1.25-openshift-4.22

USER 0

SHELL ["/bin/bash", "-c"]

RUN dnf install --assumeyes -d1 python3-pip nodejs && \
    pip3 install --upgrade pip && \
    pip3 install --upgrade setuptools && \
    pip3 install yq && \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin && \
    curl -sSL https://che-incubator.github.io/chectl/install.sh | bash -s -- --channel=next && \
    curl -sL "https://github.com/operator-framework/operator-registry/releases/download/v1.26.2/$(go env GOOS)-$(go env GOARCH)-opm" > /usr/local/bin/opm && \
    chmod +x /usr/local/bin/opm && \
    /usr/local/bin/opm version

