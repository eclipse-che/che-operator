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

# Waits until the single running operator pod logs a successful CheCluster reconciliation.
waitCheClusterReconciled() {
  echo "[INFO] Waiting for the CheCluster reconciliation"

  timeout 600s bash -c '
    while true; do
      pods=$(oc get pod -l "$1" -n "$2" -o jsonpath="{.items[*].metadata.name}" 2>/dev/null)

      if [[ $(echo "$pods" | wc -w) -eq 1 ]] && oc logs "$pods" -n "$2" 2>/dev/null | grep -q "Successfully reconciled"; then
        exit 0
      fi

      sleep 5
    done
  ' _ "$OPERATOR_POD_SELECTOR" "$OPERATOR_NAMESPACE" || {
    echo "[ERROR] Timed out waiting for the CheCluster reconciliation"
    return 1
  }

  echo "[INFO] CheCluster successfully reconciled"
}

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

# Prints the OLM catalog image built by CI for the given pull request number and the host architecture.
getPullRequestCatalogImage() {
  local pr_number="$1"
  local arch

  if [[ -z "$pr_number" ]]; then
    echo "[ERROR] Pull request number is not set" >&2
    return 1
  fi

  case "$(uname -m)" in
    x86_64|amd64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "[ERROR] Unsupported architecture: $(uname -m)" >&2
      return 1
      ;;
  esac

  echo "quay.io/eclipse/eclipse-che-olm-catalog:pr-${pr_number}-${arch}"
}

# Prints the bundle names of the given catalog image, sorted by version in ascending order.
# '--skip-tls-verify' allows reading catalog images from the in-cluster image registry,
# whose route certificate the local machine does not trust.
getBundleNamesFromCatalogImage() {
  local image="$1"

  opm render "$image" --skip-tls-verify | jq -r 'select(.schema == "olm.bundle") | .name' | sort -V
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
  local image="$1"

  opm render "$image" --skip-tls-verify | jq -r 'select(.schema == "olm.package") | .defaultChannel'
}

# Exposes the in-cluster image registry on a route and makes the cluster nodes trust its certificate,
# so that images built from sources can be pulled by the nodes. Waits for the cluster to apply it.
exposeAndTrustOpenShiftImageRegistry() {
  mkdir -p "$CA_CERT_DIR"

  # Allows pulling and pushing images in the operator namespace without credentials.
  oc policy add-role-to-user system:image-builder system:anonymous -n "$OPERATOR_NAMESPACE"
  oc policy add-role-to-user system:image-builder system:unauthenticated -n "$OPERATOR_NAMESPACE"

  oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
  until oc get route default-route -n "$IMAGE_REGISTRY_NAMESPACE" &>/dev/null; do sleep 2; done

  oc get secret router-certs-default -n "$INGRESS_NAMESPACE" -o go-template='{{index .data "tls.crt"}}' \
    | base64 -d > "${CA_CERT_DIR}/ca.crt"

  # Apply instead of delete and create, so the ConfigMap referenced by the cluster
  # image config never disappears and an unchanged certificate does not trigger reconfiguration.
  oc create configmap openshift-registry -n "$CLUSTER_CONFIG_NAMESPACE" \
    --from-file="${IMAGE_REGISTRY_ROUTE_HOST}=${CA_CERT_DIR}/ca.crt" \
    --dry-run=client -o yaml | oc apply -f -

  oc patch image.config.openshift.io/cluster --patch '{"spec":{"additionalTrustedCA":{"name":"openshift-registry"}}}' --type=merge

  waitClusterStable
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
  shift 2

  oc delete buildconfig "$name" --ignore-not-found=true -n "$OPERATOR_NAMESPACE"
  oc delete imagestreamtag "${name}:latest" --ignore-not-found=true -n "$OPERATOR_NAMESPACE"
  oc new-build --binary --strategy docker --name "$name" -n "$OPERATOR_NAMESPACE"
  oc start-build "$name" --from-dir "$dir" -n "$OPERATOR_NAMESPACE" --wait "$@"
}

# Builds the operator image from the repository sources.
buildOperatorImageFromSources() {
  echo "[INFO] Building the operator image from sources"

  buildImageInCluster "$IMAGESTREAM_OPERATOR_NAME" "$REPO_ROOT"
}

# Builds the bundle image of SOURCES_CHANNEL, pointing to the operator image built from sources.
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

# Builds the catalog image of SOURCES_CHANNEL, containing only the bundle image built from sources.
buildCatalogImageFromSources() {
  echo "[INFO] Building the catalog image from sources"

  local catalog_root_dir=$(dirname "$CATALOG_DIR")
  local bundle_name=$(make bundle-name CHANNEL="$SOURCES_CHANNEL")

  # Start from an empty directory, so no bundles are left from a previously built channel.
  rm -rf "$CATALOG_DIR"
  mkdir -p "$CATALOG_DIR"

  cat > "${CATALOG_DIR}/package.yaml" <<EOF
schema: olm.package
name: ${CHE_PACKAGE_NAME}
defaultChannel: ${SOURCES_CHANNEL}
EOF

  cat > "${CATALOG_DIR}/channel.yaml" <<EOF
schema: olm.channel
package: ${CHE_PACKAGE_NAME}
name: ${SOURCES_CHANNEL}
entries:
  - name: ${bundle_name}
EOF

  make bundle-render \
    CHANNEL="$SOURCES_CHANNEL" \
    BUNDLE_NAME="$bundle_name" \
    CATALOG_DIR="$CATALOG_DIR" \
    BUNDLE_IMG="$IMAGESTREAM_BUNDLE_IMAGE"

  cp "${REPO_ROOT}/olm-catalog/index.Dockerfile" "${catalog_root_dir}/Dockerfile"
  sed -i 's|ADD olm-catalog/${CHANNEL}|ADD '"$(basename "$CATALOG_DIR")"'|g' "${catalog_root_dir}/Dockerfile"

  buildImageInCluster "$IMAGESTREAM_CATALOG_NAME" "$catalog_root_dir" --build-arg CHANNEL="$SOURCES_CHANNEL"
}
