#!/bin/bash
#
# Copyright (c) 2019-2025 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

set -e

# https://www.eclipse.org/legal/licenses/#approved
allowed_licenses=(
  "Adobe-Glyph"
  "Apache-1.0"
  "Apache-1.1"
  "Apache-2.0"
  "Artistic-2.0"
  "BlueOak-1.0.0"
  "BSL-1.0"
  "BSD-2-Clause"
  "BSD-2-Clause-FreeBSD"
  "BSD-2-Clause-Views"
  "BSD-3-Clause"
  "BSD-4-Clause"
  "0BSD"
  "CDDL-1.0"
  "CDDL-1.1"
  "CPL-1.0"
  "CC-BY-3.0"
  "CC-BY-4.0"
  "CC-BY-2.5"
  "CC-BY-SA-3.0"
  "CC-BY-SA-4.0"
  "CC0-1.0"
  "WTFPL"
  "EPL-1.0"
  "EPL-2.0"
  "EUPL-1.1"
  "EUPL-1.2"
  "FTL"
  "GFDL-1.3"
  "LGPL-2.1"
  "LGPL-2.1-or-later"
  "LGPL-3.0"
  "LGPL-3.0-or-later"
  "LGPL-2.0"
  "LGPL-2.0-or-later"
  "IPL-1.0"
  "ISC"
  "MIT"
  "MIT-0"
  "MPL-1.1"
  "MPL-2.0"
  "NTP"
  "OpenSSL"
  "PHP-3.01"
  "PostgreSQL"
  "OFL-1.1"
  "UNLICENSE"
  "Unicode-DFS-2015"
  "Unicode-DFS-2016"
  "Unicode-TOU"
  "UPL-1.0"
  "W3C"
  "W3C-20150513"
  "W3C-19980720"
  "X11"
  "Zlib"
  "ZPL-2.1"
)

# replaces to have a correct link for clearlydefined.io api request
declare -A replaced_modules=(
  # https://github.com/open-telemetry/opentelemetry-go-contrib/commit/bc53d2b4eb4de79471bc54f64a5c3dcefa8720d7#
  ["go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0"]="github.com/open-telemetry/opentelemetry-go-contrib v1.35.0"
  # https://github.com/open-telemetry/opentelemetry-go-contrib/commit/bc53d2b4eb4de79471bc54f64a5c3dcefa8720d7#
  ["go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0"]="github.com/open-telemetry/opentelemetry-go-contrib v1.35.0"
  # https://github.com/census-instrumentation/opencensus-go/commits/v0.23.0/
  ["go.opencensus.io v0.23.0"]="census-instrumentation/opencensus-go 49838f207d61097fc0ebb8aeef306913388376ca"
  # https://github.com/sean-/seed/tree/e2103e2c35297fb7e17febb81e49b312087a2372
  ["github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529"]="sean-/seed e2103e2c35297fb7e17febb81e49b312087a2372"
)

# replaces to have a correct link for clearlydefined.io api request
declare -A replaced_paths=(
  ["go.starlark.net"]="github.com/google/starlark-go"
  ["gotest.tools"]="github.com/gotestyourself/gotest.tools"
  ["gioui.org"]="github.com/gioui/gio"
)

# replaces to have a correct link for clearlydefined.io api request
declare -A replaced_api_suffix=(
  ["census-instrumentation/opencensus-go"]="git/github"
  ["sean-/seed"]="git/github"
)

# Exceptions for dependencies that are not yet harvested in clearlydefined.io
# License must be checked manually
declare -A ignored_paths=(
  ["github.com/decred/dcrd/dcrec/secp256k1/v4"]="Harvesting is in progress"
)

declare -A ignored_paths_license=(
  ["github.com/decred/dcrd/dcrec/secp256k1/v4"]="ISC"
)

retryUrl() {
    url=$1

    body=""
    max_retries=5
    for ((i=1; i<=max_retries; i++)); do
      response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$url")
      body=$(echo "$response" | sed -e 's/HTTPSTATUS\:.*//g')
      status=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
      if [[ "$status" == "200" ]] || [[ "$status" == "404" ]]; then
        break
      else
        sleep 5s
      fi
    done

    echo "$body"
}

go list -m -mod=mod all | while read -r module; do
    # ignore the first dependency which is the current module
    if [[ "$module" == "github.com/eclipse-che/che-operator" ]]; then
        continue
    fi

    # respect the replace directive in go.mod file
    if [[ "${module}" == *"=>"* ]]; then
        module="${module#*=> }"
    fi

    orig_module="$module"
    if [[ -v replaced_modules["$orig_module"] ]]; then
      module=${replaced_modules[$orig_module]}
    fi

    path=$(echo "$module" | awk '{print $1}')

    if [[ -v ignored_paths["$path"] ]]; then
      license="${ignored_paths_license["$path"]}"
      reason="${ignored_paths["$path"]}"

      printf "%-7s %-70s %-25s %-10s %s\n" "[WARN]" "$orig_module" "$license" "N/A" "$reason"

      continue
    fi

    path=$(echo "$module" | awk '{print $1}')
    if [[ -v replaced_paths["$path"] ]]; then
      path=${replaced_paths[$path]}
    fi

    version=$(echo "$module" | awk '{print $2}')

    api_suffix="go/golang"
    if [[ -v replaced_api_suffix["$path"] ]]; then
      api_suffix=${replaced_api_suffix[$path]}
    fi

    orig_url="https://api.clearlydefined.io/definitions/${api_suffix}/${path}/${version}"
    url=$orig_url

    score=""
    body=$(retryUrl "$url")
    if [[ ! -z "$body" ]]; then
      score=$(echo "$body" | jq -r '.scores.effective')
    fi

    # try a shorter path if the first one returns null
    while [[ "$score" == "" ]] || [[ "$score" == "null" ]] || [[ "$score" == "35" ]]; do
        # remove the last part of the path
        path="${path%/*}"
        old_url=$url
        url="https://api.clearlydefined.io/definitions/go/golang/${path}/${version}"

        # if the path is the same as the old one, break to avoid infinite loop
        if [[ "$url" == "$old_url" ]]; then
            score="N/A"
            break
        fi

        # get the score again
        score=""
        body=$(retryUrl "$url")
        if [[ ! -z "$body" ]]; then
          score=$(echo "$body" | jq -r '.scores.effective')
        fi
    done

    if [[ $score == "N/A" ]]; then
      printf "%-7s %-70s %-25s %-10s %s\n" "[ERROR]" "$orig_module" "N/A" "N/A" "$orig_url"
      exit 1
    fi


    # analyze the license
    license=$(curl -s "$url" | jq -r '.licensed.declared')
    license="${license%% AND*}"

    license_approved=false
    for allowed_license in "${allowed_licenses[@]}"; do
      if [[ "$allowed_license" == "$license" ]]; then
        license_approved=true
        break
      fi
    done

    if [[ $license_approved == "false" ]]; then
      printf "%-7s %-70s %-25s %-10s %s\n" "[ERROR]" "$orig_module" "$license" "$score" "$url"
      exit 1
    fi

    # analyze the score
    if  (( score < 65 )); then
      printf "%-7s %-70s %-25s %-10s %s\n" "[ERROR]" "$orig_module" "$license" "$score" "$url"
      exit 1
    fi

    result="[OK]"
    if  (( score < 75 )); then
      result="[WARN]"
    fi
    printf "%-7s %-70s %-25s %-10s %s\n" "$result" "$orig_module" "$license" "$score" "$url"

    sleep 0.1s
done

echo "[INFO] All dependencies are defined correctly."
