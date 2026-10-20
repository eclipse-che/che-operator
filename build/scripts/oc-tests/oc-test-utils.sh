#!/bin/bash
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

# Prints the pull request number of the current branch: $PULL_NUMBER if set, otherwise detected by 'gh'.
getCurrentPullRequestNumber() {
  local pr_number

  if [[ -n "${PULL_NUMBER:-}" ]]; then
    pr_number="$PULL_NUMBER"
  elif ! pr_number=$(gh pr view --json number --jq '.number' 2>/dev/null) || [[ -z "$pr_number" ]]; then
    echo "[ERROR] Failed to detect the pull request number with 'gh'" >&2
    return 1
  fi

  echo "$pr_number"
}

# Prints the OLM catalog image built by CI for the given pull request number and the cluster architecture.
getPullRequestCatalogImage() {
  local pr_number="$1"
  local arch

  if [[ -z "$pr_number" ]]; then
    echo "[ERROR] Pull request number is not set" >&2
    return 1
  fi

  arch=$(oc get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')

  case "$arch" in
    amd64|arm64) ;;
    *)
      echo "[ERROR] Unsupported cluster architecture: '${arch}'" >&2
      return 1
      ;;
  esac

  echo "quay.io/eclipse/eclipse-che-olm-catalog:pr-${pr_number}-${arch}"
}

# Prints the bundle names of the given catalog image, sorted by version in ascending order.
# '--skip-tls-verify' allows reading catalog images from the in-cluster image registry,
# whose route certificate the local machine does not trust.
getBundleNamesFromCatalogImage() {
  opm render "$1" --skip-tls-verify | jq -r 'select(.schema == "olm.bundle") | .name' | sort -V
}

# Prints the name of the latest bundle of the given catalog image.
getLatestBundleNameFromCatalogImage() {
  getBundleNamesFromCatalogImage "$1" | tail -n 1
}

# Prints the name of the second latest bundle of the given catalog image.
getSecondLatestBundleNameFromCatalogImage() {
  local bundles

  bundles=$(getBundleNamesFromCatalogImage "$1")

  if [[ $(echo "$bundles" | wc -l) -lt 2 ]]; then
    echo "[ERROR] Catalog image '$1' contains less than two bundles" >&2
    return 1
  fi

  echo "$bundles" | tail -n 2 | head -n 1
}

# Prints the default channel of the package in the given catalog image.
getDefaultChannelFromCatalogImage() {
  opm render "$1" --skip-tls-verify | jq -r 'select(.schema == "olm.package") | .defaultChannel'
}

# Exposes the in-cluster image registry on a route and makes the cluster nodes trust its certificate,
# so that images built from sources can be pulled by the nodes. Waits for the cluster to apply it.
exposeAndTrustOpenShiftImageRegistry() {
  mkdir -p "$CA_CERT_DIR"

  oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
  until oc get route default-route -n "$IMAGE_REGISTRY_NAMESPACE" &>/dev/null; do sleep 2; done

  oc get secret router-certs-default -n "$INGRESS_NAMESPACE" -o go-template='{{index .data "tls.crt"}}' \
    | base64 --decode > "${CA_CERT_DIR}/ca.crt"

  # Apply instead of delete and create, so the ConfigMap referenced by the cluster
  # image config never disappears and an unchanged certificate does not trigger reconfiguration.
  oc create configmap openshift-registry -n "$CLUSTER_CONFIG_NAMESPACE" \
    --from-file="${IMAGE_REGISTRY_ROUTE_HOST}=${CA_CERT_DIR}/ca.crt" \
    --dry-run=client -o yaml | oc apply -f -

  oc patch image.config.openshift.io/cluster --patch '{"spec":{"additionalTrustedCA":{"name":"openshift-registry"}}}' --type=merge

  waitClusterStable
}

# Logs in to the OpenShift image registry, so that 'opm' can pull the images built from sources.
# 'opm' reads credentials only from the Docker config, so they are saved into $DOCKER_CONFIG.
# The session token is used by default; '--auth-basic' is avoided, since it splits on the first colon
# and breaks for user names containing one, e.g. 'kube:admin'.
loginToOpenShiftImageRegistry() {
  oc registry login \
    --registry="$IMAGE_REGISTRY_ROUTE_HOST" \
    --insecure=true \
    --to="${DOCKER_CONFIG}/config.json"
}

# Waits until all cluster operators are stable and all machine config pools are updated.
waitClusterStable() {
  echo "[INFO] Waiting for the cluster to apply the configuration changes"

  # The minimum stable period gives the operators time to pick up the changes,
  # otherwise the cluster may be reported as stable before the reconfiguration starts.
  oc adm wait-for-stable-cluster --minimum-stable-period=1m --timeout=30m
  oc wait machineconfigpool --all --for=condition=Updated=True --timeout=30m

  echo "[INFO] The cluster is stable"
}

# Builds an image in the operator namespace from the given directory and pushes it to the given image stream.
buildImageInCluster() {
  local name="$1"
  local dir="$2"

  oc delete buildconfig "$name" --ignore-not-found=true -n "$OPERATOR_NAMESPACE"
  oc delete imagestreamtag "${name}:latest" --ignore-not-found=true -n "$OPERATOR_NAMESPACE"
  oc new-build --binary --strategy docker --name "$name" -n "$OPERATOR_NAMESPACE"
  oc start-build "$name" --from-dir "$dir" -n "$OPERATOR_NAMESPACE" --wait
}

buildOperatorImageFromSources() {
  echo "[INFO] Building the operator image from sources"

  buildImageInCluster "$IMAGESTREAM_OPERATOR_NAME" "$REPO_ROOT"
}

buildBundleImageFromSources() {
  echo "[INFO] Building the bundle image from sources"

  # Start from an empty directory, so no files are left from a previously built channel.
  rm -rf "$BUNDLE_DIR"
  mkdir -p "$BUNDLE_DIR"

  cp -r "${REPO_ROOT}/bundle/${SOURCES_CHANNEL}/eclipse-che/." "$BUNDLE_DIR"
  mv "${BUNDLE_DIR}/bundle.Dockerfile" "${BUNDLE_DIR}/Dockerfile"

  yq -rYi '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = "'"${IMAGESTREAM_OPERATOR_IMAGE}"'"' \
    "${BUNDLE_DIR}/manifests/che-operator.clusterserviceversion.yaml"

  buildImageInCluster "$IMAGESTREAM_BUNDLE_NAME" "$BUNDLE_DIR"
}

buildCatalogImageFromSources() {
  echo "[INFO] Building the catalog image from sources"

  CHANNEL_DIR="$CATALOG_DIR/$SOURCES_CHANNEL"
  local bundle_name=$("$MAKE_BIN" bundle-name CHANNEL="$SOURCES_CHANNEL")

  # Start from an empty directory, so no bundles are left from a previously built channel.
  rm -rf "$CHANNEL_DIR"
  mkdir -p "$CHANNEL_DIR"

  cat > "${CHANNEL_DIR}/package.yaml" <<EOF
schema: olm.package
name: ${CHE_PACKAGE_NAME}
defaultChannel: ${SOURCES_CHANNEL}
EOF

  cat > "${CHANNEL_DIR}/channel.yaml" <<EOF
schema: olm.channel
package: ${CHE_PACKAGE_NAME}
name: ${SOURCES_CHANNEL}
entries:
  - name: ${bundle_name}
EOF

  # Use the 'opm' found on PATH instead of letting the Makefile download one, which requires Go.
  "$MAKE_BIN" bundle-render \
    OPM="$(command -v opm)" \
    CHANNEL="$SOURCES_CHANNEL" \
    BUNDLE_NAME="$bundle_name" \
    CATALOG_DIR="$CHANNEL_DIR" \
    BUNDLE_IMG="$IMAGESTREAM_BUNDLE_IMAGE"

  sed 's|ADD olm-catalog/${CHANNEL}|ADD /'"$SOURCES_CHANNEL"'|g' \
    "${REPO_ROOT}/olm-catalog/index.Dockerfile" > "$(dirname "$CHANNEL_DIR")/Dockerfile"

  buildImageInCluster "$IMAGESTREAM_CATALOG_NAME" "$(dirname "$CHANNEL_DIR")"
}
