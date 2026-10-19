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

# A single entry point for installing or updating Eclipse Che on an OpenShift
# cluster for testing purpose.

set -eo pipefail

export TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Check if the OpenShift CLI (`oc`) is installed.
if ! command -v "oc" &>/dev/null; then
  echo "[ERROR] 'oc' is not installed"
  exit 1
fi

# Verify that the user is logged in and can access the OpenShift cluster.
if ! oc whoami &>/dev/null; then
  echo "[ERROR] Failed to access the cluster, log in first: 'oc login'"
  exit 1
fi

# 'opm' reads registry credentials only from the Docker config. A copy of the user's one is used,
# so logging in to the OpenShift image registry does not modify it.
mkdir -p "$TMP_DIR/docker"
if [[ -f "${DOCKER_CONFIG:-$HOME/.docker}/config.json" ]]; then
  cp "${DOCKER_CONFIG:-$HOME/.docker}/config.json" "$TMP_DIR/docker/config.json"
fi
export DOCKER_CONFIG="$TMP_DIR/docker"

export MARKETPLACE_NAMESPACE="openshift-marketplace"
export IMAGE_REGISTRY_NAMESPACE="openshift-image-registry"
export INGRESS_NAMESPACE="openshift-ingress"
export CLUSTER_CONFIG_NAMESPACE="openshift-config"
export OPERATOR_NAMESPACE="openshift-operators"
export CHE_NAMESPACE="eclipse-che"

export TARGET_CATALOG_SOURCE_NAME="eclipse-che-target"
export CHE_SUBSCRIPTION_NAME="eclipse-che"
export CHE_PACKAGE_NAME="eclipse-che"
export OPERATOR_POD_SELECTOR="app.kubernetes.io/component=che-operator"
export SUPPORTED_CHANNELS=(next stable)

export BUNDLE_DIR="$TMP_DIR/bundle"
export CATALOG_DIR="$TMP_DIR/olm-catalog"
export CA_CERT_DIR="$TMP_DIR/cert"

export IMAGESTREAM_OPERATOR_NAME="eclipse-che-operator"
export IMAGESTREAM_BUNDLE_NAME="eclipse-che-bundle"
export IMAGESTREAM_CATALOG_NAME="eclipse-che-catalog"

export CLUSTER_APPS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
if [[ -z "${CLUSTER_APPS_DOMAIN:-}" ]]; then
  echo "[ERROR]: Failed to detect cluster apps domain" >&2
  exit 1
fi

export IMAGE_REGISTRY_ROUTE_HOST="default-route-openshift-image-registry.$CLUSTER_APPS_DOMAIN"
export IMAGESTREAM_OPERATOR_IMAGE="$IMAGE_REGISTRY_ROUTE_HOST/$OPERATOR_NAMESPACE/$IMAGESTREAM_OPERATOR_NAME:latest"
export IMAGESTREAM_BUNDLE_IMAGE="$IMAGE_REGISTRY_ROUTE_HOST/$OPERATOR_NAMESPACE/$IMAGESTREAM_BUNDLE_NAME:latest"
export IMAGESTREAM_CATALOG_IMAGE="$IMAGE_REGISTRY_ROUTE_HOST/$OPERATOR_NAMESPACE/$IMAGESTREAM_CATALOG_NAME:latest"

export REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
source "${REPO_ROOT}/build/scripts/oc-tests/oc-test-utils.sh"

# The Makefile relies on .ONESHELL, which requires GNU make 3.82+.
# macOS ships GNU make 3.81; a newer one is installed by Homebrew as 'gmake'.
if [[ -n "${MAKE:-}" ]]; then
  MAKE_BIN="$MAKE"
elif command -v gmake &>/dev/null; then
  MAKE_BIN="gmake"
else
  MAKE_BIN="make"
fi

init() {
  FROM="none"
  unset FROM_OPTION
  unset FROM_PR_NUMBER
  unset FROM_SOURCES_CHANNEL

  TO="none"
  unset TO_OPTION
  unset TO_PR_NUMBER
  unset TO_SOURCES_CHANNEL

  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      --install-from)
        FROM_OPTION="$2"
        parseOptionValue "$1" FROM "$2"
        shift
        ;;
      --update-to)
        TO_OPTION="$2"
        parseOptionValue "$1" TO "$2"
        shift
        ;;
      --help|-h)
        printUsage
        exit 0
        ;;
      *)
        echo "[ERROR] Unknown option: $1"
        printUsage
        exit 1
        ;;
    esac

    shift
  done

  if [[ "${FROM}" == "none" && "${TO}" == "none" ]]; then
    echo "[ERROR] At least one of '--install-from' or '--update-to' options is required"
    printUsage
    exit 1
  fi
}

# Parses the value of an option into the variables with the given prefix.
# Arguments:
#   $1 - option name, used in error messages, e.g. '--install-from'
#   $2 - variable prefix, e.g. 'FROM'
#   $3 - option value, e.g. 'pr', 'pr=2225', 'pr=current', 'next', 'sources=next'
# Sets:
#   <prefix>                 - the target type: pr, next, stable, sources or none
#   <prefix>_PR_NUMBER       - the pull request number, for the 'pr' target
#   <prefix>_SOURCES_CHANNEL - the channel to build from, for the 'sources' target
parseOptionValue() {
  local option="$1"
  local prefix="$2"
  local value="$3"

  if [[ -z "${value}" || "${value}" == -* ]]; then
    echo "[ERROR] Missing value for the '${option}' option."
    printUsage
    exit 1
  fi

  local target="${value%%=*}"
  local argument=""
  if [[ "${value}" == *=* ]]; then
    argument="${value#*=}"
  fi

  case "${target}" in
    pr)
      # The pull request of the current branch is used by default.
      if [[ -z "${argument}" || "${argument}" == "current" ]]; then
        local pr_number
        if ! pr_number=$(getCurrentPullRequestNumber); then
          echo "[ERROR] Failed to detect the pull request of the current branch for '${option} ${value}'. Set the PULL_NUMBER environment variable."
          printUsage
          exit 1
        fi

        argument="${pr_number}"
      fi

      if [[ ! "${argument}" =~ ^[0-9]+$ ]]; then
        echo "[ERROR] Invalid '${option}' value: '${value}'. Expected 'pr', 'pr=<number>' or 'pr=current'."
        printUsage
        exit 1
      fi

      printf -v "${prefix}_PR_NUMBER" '%s' "${argument}"
      ;;
    sources)
      if [[ ! " ${SUPPORTED_CHANNELS[*]} " =~ " ${argument} " || -z "${argument}" ]]; then
        echo "[ERROR] Invalid '${option}' value: '${value}'. Expected 'sources=<channel>', where <channel> is one of: $(supportedChannelsList)."
        printUsage
        exit 1
      fi

      printf -v "${prefix}_SOURCES_CHANNEL" '%s' "${argument}"
      ;;
    next|stable|none)
      if [[ "${value}" == *=* ]]; then
        echo "[ERROR] Invalid '${option}' value: '${value}'. The '${target}' target does not accept an argument."
        printUsage
        exit 1
      fi
      ;;
    *)
      echo "[ERROR] Unknown '${option}' value: '${value}'. Must be one of: pr, pr=<number>, pr=current, next, stable, sources=<channel>, none"
      printUsage
      exit 1
      ;;
  esac

  printf -v "${prefix}" '%s' "${target}"
}

checkPrerequisites() {
  local tool
  local failed=false

  for tool in jq yq opm; do
    if ! command -v "$tool" &>/dev/null; then
      echo "[ERROR] '${tool}' is not installed"
      failed=true
    fi
  done

  if command -v yq &>/dev/null && ! yq --help 2>&1 | grep -q -- '--yaml-output'; then
    echo "[ERROR] The Python 'yq' (https://github.com/kislyuk/yq) is required: 'pip install yq' or 'brew install python-yq'"
    failed=true
  fi

  local make_version=$("$MAKE_BIN" --version 2>/dev/null | head -n 1 | awk '/GNU Make/ {print $3}')
  if [[ -z "$make_version" || "$(printf '%s\n' 3.82 "$make_version" | sort -V | head -n 1)" != "3.82" ]]; then
    echo "[ERROR] GNU make 3.82+ is required, found '${make_version:-unknown}': 'brew install make' or set MAKE"
    failed=true
  fi

  if [[ "$failed" == "true" ]]; then
    exit 1
  fi
}

printUsage() {
  echo "Test Eclipse Che install or update."
  echo
  echo "Usage:"
  echo -e "  $0 [--install-from TARGET] [--update-to TARGET]"
  echo
  echo "OPTIONS:"
  echo -e "  --install-from TARGET  Eclipse Che installation to install as the source of the update"
  echo -e "                         (default: none)"
  echo -e "  --update-to TARGET     Eclipse Che installation to update the source to"
  echo -e "                         (default: none)"
  echo -e "  -h, --help             Show this help message"
  echo
  echo "TARGETS:"
  echo -e "  pr=<number>       - the Eclipse Che build from a pull request, where <number>"
  echo -e "                      is the pull request number, e.g. --update-to pr=2225"
  echo -e "  pr, pr=current    - the Eclipse Che build from the pull request of the current"
  echo -e "                      branch, detected with the 'gh' CLI or taken from the"
  echo -e "                      PULL_NUMBER environment variable"
  echo -e "  next              - the Eclipse Che build from the next channel; with"
  echo -e "                      '--install-from next --update-to next' the previous next version is"
  echo -e "                      installed and then updated to the latest one"
  echo -e "  stable            - the Eclipse Che build from the stable channel; with"
  echo -e "                      '--install-from stable --update-to stable' the previous stable version is"
  echo -e "                      installed and then updated to the latest one"
  echo -e "  sources=<channel> - the Eclipse Che build created from the local sources of the"
  echo -e "                      given channel and pushed to the image registry of the"
  echo -e "                      cluster, where <channel> is one of: $(supportedChannelsList)"
  echo -e "  none              - --install-from: the Eclipse Che installation that is already"
  echo -e "                      deployed on the cluster; --update-to: do not update Eclipse Che"
  echo
  echo "Examples:"
  echo -e "  $0 --install-from pr=2225 --update-to sources=next"
  echo -e "  $0 --install-from pr --update-to sources=next"
  echo -e "  $0 --install-from pr --update-to pr=1900"
  echo -e "  $0 --install-from next --update-to pr"
  echo -e "  $0 --install-from next --update-to stable"
  echo -e "  $0 --install-from next --update-to next"
  echo -e "  $0 --install-from stable --update-to next"
  echo -e "  $0 --install-from stable --update-to stable"
  echo -e "  $0 --install-from sources=next --update-to sources=stable"
  echo -e "  $0 --install-from sources=stable --update-to pr=2225"
  echo -e "  $0 --update-to pr=2225"
  echo -e "  $0 --install-from pr=2225"
}

supportedChannelsList() {
  local channels=$(printf ", %s" "${SUPPORTED_CHANNELS[@]}")
  echo "${channels:2}"
}

installFromNext() {
  local catalog_image="quay.io/eclipse/eclipse-che-olm-catalog:next"
  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv

  # When updating to the next channel as well, install the previous next version,
  # so that there is a newer one to update to.
  if [[ "${TO:-}" == "next" ]]; then
    desired_csv=$(getSecondLatestBundleNameFromCatalogImage "$catalog_image")
  else
    desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")
  fi

  installFrom "$desired_channel" "$desired_csv" "$catalog_image"
}

installFromStable() {
  local catalog_image="quay.io/eclipse/eclipse-che-olm-catalog:stable"
  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv

  # When updating to the stable channel as well, install the previous stable version,
  # so that there is a newer one to update to.
  if [[ "${TO:-}" == "stable" ]]; then
    desired_csv=$(getSecondLatestBundleNameFromCatalogImage "$catalog_image")
  else
    desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")
  fi

  installFrom "$desired_channel" "$desired_csv" "$catalog_image"
}

installFromPullRequest() {
  local catalog_image
  catalog_image=$(getPullRequestCatalogImage "$FROM_PR_NUMBER")

  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")

  installFrom "$desired_channel" "$desired_csv" "$catalog_image"
}

installFromSources() {
  SOURCES_CHANNEL="${FROM_SOURCES_CHANNEL}"

  echo "[INFO] Preparing the OpenShift image registry to host the images built from sources"

  exposeAndTrustOpenShiftImageRegistry
  loginToOpenShiftImageRegistry
  buildOperatorImageFromSources
  buildBundleImageFromSources
  buildCatalogImageFromSources

  local catalog_image="$IMAGESTREAM_CATALOG_IMAGE"
  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")

  installFrom "$desired_channel" "$desired_csv" "$catalog_image"
}

upgradeToPullRequest() {
  local catalog_image
  catalog_image=$(getPullRequestCatalogImage "$TO_PR_NUMBER")

  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")

  upgradeTo "$desired_channel" "$desired_csv" "$catalog_image"
}

upgradeToNext() {
  local catalog_image="quay.io/eclipse/eclipse-che-olm-catalog:next"
  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")

  upgradeTo "$desired_channel" "$desired_csv" "$catalog_image"
}

upgradeToStable() {
  local catalog_image="quay.io/eclipse/eclipse-che-olm-catalog:stable"
  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")

  upgradeTo "$desired_channel" "$desired_csv" "$catalog_image"
}

upgradeToSources() {
  SOURCES_CHANNEL="${TO_SOURCES_CHANNEL}"

  echo "[INFO] Preparing the OpenShift image registry to host the images built from sources"

  exposeAndTrustOpenShiftImageRegistry
  loginToOpenShiftImageRegistry
  buildOperatorImageFromSources
  buildBundleImageFromSources
  buildCatalogImageFromSources

  local catalog_image="$IMAGESTREAM_CATALOG_IMAGE"
  local desired_channel=$(getDefaultChannelFromCatalogImage "$catalog_image")
  local desired_csv=$(getLatestBundleNameFromCatalogImage "$catalog_image")

  upgradeTo "$desired_channel" "$desired_csv" "$catalog_image"
}

installFrom() {
  local desired_channel="$1"
  local desired_csv="$2"
  local catalog_image="$3"

  echo "[INFO] Installing Eclipse Che '${desired_csv}'"

  if [[ "$FROM" == "next" ]] || [[ "$FROM" == "pr" ]] || [[ "$FROM" == "sources" && "$FROM_SOURCES_CHANNEL" == "next" ]]; then
    # Install DevWorkspace operator next version
    "$MAKE_BIN" install-devworkspace CHANNEL="fast" NAMESPACE="$OPERATOR_NAMESPACE"
  fi

  local catalog_source_namespace="$MARKETPLACE_NAMESPACE"
  if [[ "$FROM" == "sources" ]]; then
    # Use namespace where image is build
    catalog_source_namespace="$OPERATOR_NAMESPACE"
  fi

  "$MAKE_BIN" create-catalogsource \
    NAME="$TARGET_CATALOG_SOURCE_NAME" \
    NAMESPACE="$catalog_source_namespace" \
    IMAGE="$catalog_image"

  "$MAKE_BIN" create-subscription \
    NAME="$CHE_SUBSCRIPTION_NAME" \
    NAMESPACE="$OPERATOR_NAMESPACE" \
    CHANNEL="$desired_channel" \
    INSTALL_PLAN_APPROVAL="Manual" \
    PACKAGE_NAME="$CHE_PACKAGE_NAME" \
    SOURCE="$TARGET_CATALOG_SOURCE_NAME" \
    SOURCE_NAMESPACE="$catalog_source_namespace" \
    STARTING_CSV="$desired_csv"

  "$MAKE_BIN" approve-installplan SUBSCRIPTION_NAME="$CHE_SUBSCRIPTION_NAME" NAMESPACE="$OPERATOR_NAMESPACE"

  echo "[INFO] Waiting for the Eclipse Che operator pod to be running"
  "$MAKE_BIN" wait-pod-running SELECTOR="$OPERATOR_POD_SELECTOR" NAMESPACE="$OPERATOR_NAMESPACE"

  echo "[INFO] Applying CheCluster"
  oc create namespace "$CHE_NAMESPACE" --dry-run=client -o yaml | oc apply -f -
  oc get csv "$desired_csv" -n "$OPERATOR_NAMESPACE" -o json \
    | jq '.metadata.annotations["alm-examples"] | fromjson | .[] | select(.apiVersion == "org.eclipse.che/v2")' \
    | oc apply --server-side -n "$CHE_NAMESPACE" -f -

  "$MAKE_BIN" wait-eclipseche-reconciled NAMESPACE="$OPERATOR_NAMESPACE"
}

upgradeTo() {
  local desired_channel="$1"
  local desired_csv="$2"
  local catalog_image="$3"

  local installed_csv=$(oc get subscription "$CHE_SUBSCRIPTION_NAME" -n "$OPERATOR_NAMESPACE" -o jsonpath='{.status.installedCSV}')

  if [[ "${installed_csv}" == "$desired_csv" ]]; then
    echo "[ERROR] Eclipse Che version '${installed_csv}' is already installed; there is nothing to update to."
    exit 1
  fi

  echo "[INFO] Updating Eclipse Che '${desired_csv}'"

  oc delete csv "$installed_csv" -n "$OPERATOR_NAMESPACE" --ignore-not-found
  oc wait --for=delete csv/"$installed_csv" -n "$OPERATOR_NAMESPACE" --timeout=60s

  oc delete subscription "$CHE_SUBSCRIPTION_NAME" -n "$OPERATOR_NAMESPACE" --ignore-not-found
  oc wait --for=delete subscription/"$CHE_SUBSCRIPTION_NAME" -n "$OPERATOR_NAMESPACE" --timeout=60s

  oc wait pod -l "$OPERATOR_POD_SELECTOR"  -n "$OPERATOR_NAMESPACE"  --for=delete --timeout=60s

  local catalog_source_namespace="$MARKETPLACE_NAMESPACE"
  if [[ "$TO" == "sources" ]]; then
    # Use namespace where image is build
    catalog_source_namespace="$OPERATOR_NAMESPACE"
  fi

  "$MAKE_BIN" create-catalogsource \
    NAME="$TARGET_CATALOG_SOURCE_NAME" \
    NAMESPACE="$catalog_source_namespace" \
    IMAGE="$catalog_image"

  "$MAKE_BIN" create-subscription \
    NAME="$CHE_SUBSCRIPTION_NAME" \
    NAMESPACE="$OPERATOR_NAMESPACE" \
    CHANNEL="$desired_channel" \
    INSTALL_PLAN_APPROVAL="Auto" \
    PACKAGE_NAME="$CHE_PACKAGE_NAME" \
    SOURCE="$TARGET_CATALOG_SOURCE_NAME" \
    SOURCE_NAMESPACE="$catalog_source_namespace"

  echo "[INFO] Waiting for the Eclipse Che operator pod to be running"
  "$MAKE_BIN" wait-pod-running SELECTOR="$OPERATOR_POD_SELECTOR" NAMESPACE="$OPERATOR_NAMESPACE"

  "$MAKE_BIN" wait-eclipseche-reconciled NAMESPACE="$OPERATOR_NAMESPACE"
}

install() {
  case "${FROM}" in
    pr)
      installFromPullRequest
      ;;
    next)
      installFromNext
      ;;
    stable)
      installFromStable
      ;;
    sources)
      installFromSources
      ;;
    none)
      echo "[INFO] Skipping Eclipse Che install"
      ;;
    *)
      echo "[ERROR] Unknown '--install-from' value: '${FROM}'. Must be one of: pr, pr=<number>, pr=current, next, stable, sources=<channel>, none"
      printUsage
      exit 1
      ;;
  esac
}

upgrade() {
  case "${TO}" in
    pr)
      upgradeToPullRequest
      ;;
    next)
      upgradeToNext
      ;;
    stable)
      upgradeToStable
      ;;
    sources)
      upgradeToSources
      ;;
    none)
      echo "[INFO] Skipping Eclipse Che upgrade"
      ;;
    *)
      echo "[ERROR] Unknown '--update-to' value: '${TO}'. Must be one of: pr, pr=<number>, pr=current, next, stable, sources=<channel>, none"
      printUsage
      exit 1
      ;;
  esac
}

main() {
  install
  upgrade

  echo "[INFO] Completed"
}

checkPrerequisites
init "$@"
main
