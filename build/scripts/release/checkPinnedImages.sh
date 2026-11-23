#!/usr/bin/env bash
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
# Checks that every image referenced by the CSV's RELATED_IMAGE_* variables can be pinned with
# an OpenShift PinnedImageSet (machineconfiguration.openshift.io/v1). Each value must be a
# digest reference accepted by the API, and must pass `podman manifest inspect` - the command
# the machine config daemon runs to prefetch the set.
#
# The case this check exists for is a single OCI image manifest. It is well formed and
# pullable, but `podman manifest inspect` fails on it with rc=125 ("Treating single images as
# manifest lists is not implemented"). Because the prefetch is all-or-nothing per node, one
# such image silently disables the whole set.
#
# make-release.sh calls this after addDigests.sh, on the digest-pinned stable CSV, with
# --forbid-next-tags: addDigests.sh records the tag each digest was pinned from as a commented
# `# tag:` line, and an image that came from a moving `:next` tag is rebuilt and removed behind
# the pinned digest, which breaks workspaces. The next catalog keeps those tags by design and
# does not pass the flag.
#
# Usage: checkPinnedImages.sh --csv <CSV> [--authfile <file>] [--forbid-next-tags]
#
# Requires podman and the python-yq (kislyuk) implementation. Set CONTAINERS_REGISTRIES_CONF to
# check pre-release images through a mirror.

set -Eeuo pipefail

# IMAGE_RE is the CEL rule on spec.pinnedImages[*].name (openshift/api, ImageDigestFormat),
# with the digest expression inlined. In words, a pinnable reference looks like:
#
#   host[:port]/[namespace/]name@sha256:<64 lowercase hex characters>
#
# - host: dot-separated labels, so at least one dot. A dotless host, such as localhost:5000,
#   is not accepted here nor by the API.
# - port: optional, 2 to 5 digits.
# - namespace: optional, at most one segment of up to 61 characters.
# - name: letters, digits, '_', '.' and '-'; it may be empty.
# - digest: no tag before the '@', and exactly 64 lowercase hex characters after 'sha256:'.
#
# Do not tighten any part of it: the API accepts an empty namespace segment and an empty name,
# so rejecting those here would reject references the cluster accepts.
IMAGE_RE='^([a-zA-Z0-9-]+\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9_-]{0,61}/)?[a-zA-Z0-9_.-]*@sha256:[a-f0-9]{64}$'
MAX_REFERENCE_LENGTH=447

die() {
  echo "[ERROR] $*" >&2
  exit 1
}

CSV=""
AUTHFILE=""
FORBID_NEXT_TAGS=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --csv)
      [[ $# -ge 2 ]] || die "--csv requires a value"
      CSV="$2"
      shift 2
      ;;
    --authfile)
      [[ $# -ge 2 ]] || die "--authfile requires a value"
      AUTHFILE="$2"
      shift 2
      ;;
    --forbid-next-tags)
      FORBID_NEXT_TAGS=true
      shift
      ;;
    -h | --help)
      sed -n '/^# Usage:/,/^$/s/^# \{0,1\}//p' "$0"
      exit 0
      ;;
    *) die "unknown argument: $1 (see --help)" ;;
  esac
done

[[ -n "${CSV}" ]] || die "usage: $(basename "$0") --csv <CSV> [--authfile <file>]"
[[ -r "${CSV}" ]] || die "${CSV} cannot be read"
[[ -z "${AUTHFILE}" || -r "${AUTHFILE}" ]] || die "${AUTHFILE} cannot be read"
command -v podman >/dev/null 2>&1 || die "podman is not installed"
command -v yq >/dev/null 2>&1 || die "yq is not installed; the python-yq (kislyuk) implementation is required"
yq --version 2>/dev/null | grep -q mikefarah && die "yq is the mikefarah implementation; the python-yq (kislyuk) implementation is required"

# A throwaway store, because an image already present locally makes `podman manifest inspect`
# resolve offline and hide a bad reference.
STORE=$(mktemp -d "${TMPDIR:-/tmp}/checkPinnedImages.XXXXXX")
trap 'rm -rf "${STORE}"' EXIT
mkdir -p "${STORE}/root" "${STORE}/run"

# The env list is read with python-yq, so quoting, comments, folded scalars and key order are the
# YAML parser's problem rather than this script's. Fail on an entry that cannot be pinned by value:
# valueFrom, a missing, empty or multi-line value.
RELATED_ENVS='.spec.install.spec.deployments[]?
  | .spec.template.spec
  | (.containers[]?, .initContainers[]?)
  | .env[]?
  | select((.name // "") | startswith("RELATED_IMAGE_"))'

related_count=$(yq -r "[${RELATED_ENVS}] | length" "${CSV}") ||
  die "could not parse the RELATED_IMAGE_* variables of ${CSV}"

if [[ "${related_count}" == "0" ]]; then
  die "no RELATED_IMAGE_* variables found in ${CSV}"
fi

value_from=$(yq -r "[${RELATED_ENVS} | select(.valueFrom != null)] | length" "${CSV}") ||
  die "could not parse the RELATED_IMAGE_* variables of ${CSV}"
if [[ "${value_from}" != "0" ]]; then
  die "a RELATED_IMAGE_* value uses valueFrom in ${CSV}"
fi

invalid=$(yq -r "[${RELATED_ENVS} | select((.value | type) != \"string\" or .value == \"\" or (.value | test(\"\\n\")))] | length" "${CSV}") ||
  die "could not parse the RELATED_IMAGE_* variables of ${CSV}"
if [[ "${invalid}" != "0" ]]; then
  die "a RELATED_IMAGE_* value is empty or multi-line in ${CSV}"
fi

refs=$(yq -r "${RELATED_ENVS} | .value" "${CSV}") ||
  die "could not read the RELATED_IMAGE_* values of ${CSV}"

next_origins=""
if [[ "${FORBID_NEXT_TAGS}" == true ]]; then
  # addDigests.sh keeps the source tag as a commented line, so a digest alone does not say
  # whether the image is rebuilt behind it. Without those lines the rule cannot work, so fail
  # instead of passing every image.
  grep -qE '^[[:space:]]*#[[:space:]]*tag:' "${CSV}" ||
    die "--forbid-next-tags needs the '# tag:' origin lines that addDigests.sh writes; none found in ${CSV}"
  next_origins=$(grep -E '^[[:space:]]*#[[:space:]]*tag:[[:space:]]*"?[^"[:space:]]*:next"?[[:space:]]*$' "${CSV}" | sort -u || true)
fi

# Count distinct references, then check each one once.
echo "[INFO] checking $(sort -u <<<"${refs}" | wc -l) distinct image references from ${CSV}"

AUTHARGS=()
[[ -n "${AUTHFILE}" ]] && AUTHARGS=(--authfile "${AUTHFILE}")

failures=0
forbidden=0
while IFS= read -r ref; do
  [[ -n "${ref}" ]] || continue

  if (( ${#ref} > MAX_REFERENCE_LENGTH )) || [[ ! "${ref}" =~ ${IMAGE_RE} ]]; then
    echo "[FAIL] ${ref}"
    echo "       not a digest reference accepted by the PinnedImageSet API (expected host/path@sha256:<64 hex>)"
    failures=$((failures + 1))
    continue
  fi

  if ! output=$(podman --log-level error --root "${STORE}/root" --runroot "${STORE}/run" \
    manifest inspect "${AUTHARGS[@]}" "${ref}" 2>&1); then
    # The single-manifest error embeds the whole manifest JSON; keep the head and the tail,
    # where the cause is.
    output="${output//$'\n'/ }"
    (( ${#output} > 300 )) && output="${output:0:90} ... ${output: -180}"
    echo "[FAIL] ${ref}"
    echo "       podman manifest inspect failed: ${output}"
    failures=$((failures + 1))
  fi
done < <(sort -u <<<"${refs}")

if [[ -n "${next_origins:-}" ]]; then
  while IFS= read -r origin; do
    origin="${origin#*tag:}"
    origin="${origin//\"/}"
    origin="${origin#"${origin%%[![:space:]]*}"}"
    echo "[FAIL] ${origin}"
    echo "       pinned from a ':next' tag, which moves and leaves the digest behind"
    forbidden=$((forbidden + 1))
  done <<< "${next_origins}"
fi

if (( failures > 0 || forbidden > 0 )); then
  summary=""
  (( failures > 0 )) && summary="${failures} image(s) failed the pinned-image check"
  (( forbidden > 0 )) && summary="${summary:+${summary}; }${forbidden} image(s) were pinned from a ':next' tag"
  die "${summary}"
fi

echo "[INFO] every image can be pinned with a PinnedImageSet"
