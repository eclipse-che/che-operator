# Openshift opm bundle image. Should be used to build two images: preview stable and preview nightly Eclipse Che.

FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=eclipse-che-preview-openshift
LABEL operators.operatorframework.io.bundle.channels.v1=stable,nightly
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable

COPY stable/manifests /manifests/
COPY stable/metadata /metadata/
