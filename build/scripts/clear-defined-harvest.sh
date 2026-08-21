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

# Requests harvesting on clearlydefined.io for the Go dependencies that are
# currently marked "Harvesting is in progress" in clear-defined-test.sh.
#
# Those dependencies are ignored by the license check because clearlydefined.io
# has no definition for them yet. Submitting a harvest request tells
# clearlydefined.io to fetch and analyze them so that, once harvested, they can
# be removed from the ignore list in clear-defined-test.sh.
#
# Usage:
#   build/scripts/clear-defined-harvest.sh          # queue harvest requests
#   DRY_RUN=true build/scripts/clear-defined-harvest.sh  # only print, do not POST

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_SCRIPT="${SCRIPT_DIR}/clear-defined-test.sh"

if [[ ! -f "$TEST_SCRIPT" ]]; then
  echo "[ERROR] cannot find ${TEST_SCRIPT}"
  exit 1
fi

HARVEST_URL="https://api.clearlydefined.io/harvest"
DRY_RUN="${DRY_RUN:-false}"

# Number of attempts per harvest request (transient timeouts return HTTP 000).
HARVEST_RETRIES="${HARVEST_RETRIES:-5}"

# Extract the module paths marked "Harvesting is in progress" from the test
# script so the two lists never drift apart.
readarray -t harvest_paths < <(
  grep -oP '\["\K[^"]+(?="\]="Harvesting is in progress")' "$TEST_SCRIPT"
)

if [[ ${#harvest_paths[@]} -eq 0 ]]; then
  echo "[INFO] no dependencies marked \"Harvesting is in progress\" found."
  exit 0
fi

# Build a lookup set of the paths to harvest.
declare -A is_harvest_path=()
for p in "${harvest_paths[@]}"; do
  is_harvest_path["$p"]=1
done

# Builds a clearlydefined.io coordinate for a Go module.
# Format: go/golang/<namespace>/<name>/<version>
# The namespace is everything but the last path segment, with slashes url-encoded
# as %2F; the name is the last path segment.
buildCoordinate() {
  local path="$1"
  local version="$2"

  local name="${path##*/}"
  local namespace="${path%/*}"
  namespace="${namespace//\//%2F}"

  echo "go/golang/${namespace}/${name}/${version}"
}

# Sends a harvest request for a single coordinate and prints the HTTP status.
requestHarvest() {
  local module="$1"
  local coordinate="$2"

  if [[ "$DRY_RUN" == "true" ]]; then
    printf "%-9s %-70s %s\n" "[DRY-RUN]" "$module" "$coordinate"
    return
  fi

  local payload status result
  payload="[{\"tool\":\"package\",\"coordinates\":\"${coordinate}\"}]"

  # Retry on transient failures (e.g. timeouts return HTTP 000).
  for ((attempt = 1; attempt <= HARVEST_RETRIES; attempt++)); do
    # Do not let a single curl failure (e.g. a timeout) abort the whole run.
    status=$(curl -s -o /dev/null -w "%{http_code}" \
      --max-time 30 \
      -X POST "$HARVEST_URL" \
      -H "accept: */*" \
      -H "Content-Type: application/json" \
      -d "$payload" || true)

    if [[ "$status" == "200" || "$status" == "201" || "$status" == "202" ]]; then
      break
    fi

    (( attempt < HARVEST_RETRIES )) && sleep 2
  done

  result="[QUEUED]"
  if [[ "$status" != "200" && "$status" != "201" && "$status" != "202" ]]; then
    result="[FAILED]"
  fi

  printf "%-9s %-70s %s (HTTP %s)\n" "$result" "$module" "$coordinate" "$status"

  [[ "$result" == "[QUEUED]" ]]
}

readarray -t modules < <(go list -m -mod=mod all)

queued=0
failed=0
for module in "${modules[@]}"; do
  # respect the replace directive in go.mod file
  if [[ "${module}" == *"=>"* ]]; then
    module="${module#*=> }"
  fi

  path=$(echo "$module" | awk '{print $1}')
  version=$(echo "$module" | awk '{print $2}')

  if [[ ! -v is_harvest_path["$path"] ]]; then
    continue
  fi

  if [[ -z "$version" ]]; then
    printf "%-9s %-70s %s\n" "[SKIP]" "$path" "no version resolved"
    failed=$((failed + 1))
    continue
  fi

  coordinate=$(buildCoordinate "$path" "$version")

  if requestHarvest "$path" "$coordinate"; then
    queued=$((queued + 1))
  else
    failed=$((failed + 1))
  fi

  sleep 0.1s
done

echo "[INFO] queued ${queued} dependency(ies) for harvesting, ${failed} failed."
echo "[INFO] harvesting is asynchronous; re-run clear-defined-test.sh later to verify."

if [[ "$DRY_RUN" != "true" && $failed -gt 0 ]]; then
  exit 1
fi
