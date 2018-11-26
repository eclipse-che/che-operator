FROM golang:1.10.3 as builder
ADD . /go/src/github.com/eclipse/che-operator
RUN OOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -o /tmp/che-operator/che-operator \
    /go/src/github.com/eclipse/che-operator/cmd/che-operator/main.go

FROM alpine:3.7
COPY --from=builder /tmp/che-operator/che-operator /usr/local/bin/che-operator
COPY --from=builder /go/src/github.com/eclipse/che-operator/deploy/keycloak_provision /tmp/keycloak_provision
RUN adduser -D che-operator
USER che-operator
CMD ["che-operator"]