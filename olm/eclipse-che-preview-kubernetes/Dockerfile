FROM quay.io/operator-framework/upstream-registry-builder:v1.15.1 as builder

COPY deploy manifests/eclipse-che-preview-kubernetes
RUN ./bin/initializer -o ./bundles.db

FROM openshift/origin-base

COPY --from=builder /bin/initializer /initializer
COPY --from=builder /bin/configmap-server /bin/configmap-server
COPY --from=builder /bundles.db /bundles.db
COPY --from=builder /bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe

EXPOSE 50051

ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
