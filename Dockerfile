#
# Copyright (c) 2012-2018 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

FROM golang:1.10.3 as builder
ADD . /go/src/github.com/eclipse/che-operator
RUN cd /go/src/github.com/eclipse/che-operator && go test -v ./...
RUN OOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -o /tmp/che-operator/che-operator \
    /go/src/github.com/eclipse/che-operator/cmd/che-operator/main.go

FROM alpine:3.7
COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /go/src/github.com/eclipse/che-operator/deploy/keycloak_provision /tmp/keycloak_provision
RUN adduser -D che-operator
USER che-operator
CMD ["che-operator"]
