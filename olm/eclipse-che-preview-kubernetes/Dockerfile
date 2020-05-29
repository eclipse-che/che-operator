FROM quay.io/operator-framework/upstream-registry-builder as builder

COPY deploy manifests/eclipse-che-preview-kubernetes
RUN ./bin/initializer -o ./bundles.db

FROM openshift/origin-base

COPY --from=builder /build/bin/initializer /initializer
COPY --from=builder /build/bin/configmap-server /bin/configmap-server
COPY --from=builder /build/bundles.db /bundles.db
COPY --from=builder /build/bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe

EXPOSE 50051

ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
