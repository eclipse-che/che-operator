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
  # https://github.com/open-telemetry/opentelemetry-go-contrib/commit/a89d958e7a2575cf539ffca09657cfd90821d6e4
  ["go.opentelemetry.io/contrib/bridges/prometheus v0.65.0"]="github.com/open-telemetry/opentelemetry-go-contrib v1.40.0"
  # https://github.com/open-telemetry/opentelemetry-go-contrib/commit/a89d958e7a2575cf539ffca09657cfd90821d6e4
  ["go.opentelemetry.io/contrib/exporters/autoexport v0.65.0"]="github.com/open-telemetry/opentelemetry-go-contrib v1.40.0"
  # https://github.com/open-telemetry/opentelemetry-go-contrib/commit/a89d958e7a2575cf539ffca09657cfd90821d6e4
  ["go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.65.0"]="github.com/open-telemetry/opentelemetry-go-contrib v1.40.0"
  # https://github.com/open-telemetry/opentelemetry-go-contrib/commit/a89d958e7a2575cf539ffca09657cfd90821d6e4
  ["go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.65.0"]="github.com/open-telemetry/opentelemetry-go-contrib v1.40.0"
  # https://github.com/open-telemetry/opentelemetry-go/commit/a3a5317c5caed1656fb5b301b66dfeb3c4c944e0
  ["go.opentelemetry.io/otel/exporters/prometheus v0.62.0"]="github.com/open-telemetry/opentelemetry-go v1.40.0"
  # https://github.com/census-instrumentation/opencensus-go/commits/v0.23.0/
  ["go.opencensus.io v0.23.0"]="census-instrumentation/opencensus-go 49838f207d61097fc0ebb8aeef306913388376ca"
  # https://github.com/census-instrumentation/opencensus-go/commits/v0.24.0/
  ["go.opencensus.io v0.24.0"]="census-instrumentation/opencensus-go b1a01ee95db0e690d91d7193d037447816fae4c5"
  # https://github.com/sean-/seed/tree/e2103e2c35297fb7e17febb81e49b312087a2372
  ["github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529"]="sean-/seed e2103e2c35297fb7e17febb81e49b312087a2372"
  # https://github.com/decred/dcrd/commit/5444fa50b93dbcbd6a08c75da3eccc32490fb2b2
  ["github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.0-20210816181553-5444fa50b93d"]="decred/dcrd 5444fa50b93dbcbd6a08c75da3eccc32490fb2b2"
  # https://github.com/prometheus-operator/prometheus-operator/commit/32d1b3dfa05d070762450efe9624bb2483c782be
  ["github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.90.1"]="github.com/prometheus-operator/prometheus-operator v0.90.1"
)

# replaces to have a correct link for clearlydefined.io api request
declare -A replaced_paths=(
  ["go.starlark.net"]="github.com/google/starlark-go"
  ["gotest.tools"]="github.com/gotestyourself/gotest.tools"
  ["gioui.org"]="github.com/gioui/gio"
  # ["go.podman.io/common"]="github.com/containers/container-libs"
  # ["go.podman.io/image/v5"]="github.com/containers/container-libs"
  # ["go.podman.io/storage"]="github.com/containers/container-libs"
)

# replaces to have a correct link for clearlydefined.io api request
declare -A replaced_api_suffix=(
  ["census-instrumentation/opencensus-go"]="git/github"
  ["sean-/seed"]="git/github"
  ["decred/dcrd"]="git/github"
)

# Exceptions for dependencies that are not yet harvested in clearlydefined.io
# License must be checked manually
# https://clearlydefined.io/harvest
declare -A ignored_paths=(
  ["github.com/devfile/devworkspace-operator"]="Harvesting is in progress"
  ["github.com/go-openapi/testify/enable/yaml/v2"]="Harvesting is in progress"
  ["github.com/grpc-ecosystem/grpc-health-probe"]="Harvesting is in progress"
  ["github.com/miekg/pkcs11"]="Harvesting is in progress"
  ["github.com/openshift/api"]="Harvesting is in progress"
  ["github.com/openshift/library-go"]="Harvesting is in progress"
  ["github.com/operator-framework/api"]="Harvesting is in progress"
  ["github.com/operator-framework/operator-lifecycle-manager"]="Harvesting is in progress"
  ["github.com/operator-framework/operator-registry"]="Harvesting is in progress"
  ["github.com/redis/go-redis/extra/rediscmd/v9"]="Harvesting is in progress"
  ["github.com/redis/go-redis/extra/redisotel/v9"]="Harvesting is in progress"
  ["github.com/sigstore/fulcio"]="Harvesting is in progress"
  ["github.com/open-telemetry/opentelemetry-go-contrib"]="Harvesting is in progress"
  ["github.com/open-telemetry/opentelemetry-go"]="Harvesting is in progress"
  ["go.podman.io/common"]="Harvesting is in progress"
  ["go.podman.io/image/v5"]="Harvesting is in progress"
  ["go.podman.io/storage"]="Harvesting is in progress"
  ["golang.org/x/tools/go/packages/packagestest"]="Harvesting is in progress"
  ["google.golang.org/genproto"]="Harvesting is in progress"
  ["k8s.io/apiextensions-apiserver"]="Harvesting is in progress"
  ["k8s.io/apiserver"]="Harvesting is in progress"
  ["k8s.io/code-generator"]="Harvesting is in progress"
  ["k8s.io/component-base"]="Harvesting is in progress"
  ["k8s.io/kms"]="Harvesting is in progress"
  ["k8s.io/kube-aggregator"]="Harvesting is in progress"
  ["k8s.io/kube-openapi"]="Harvesting is in progress"
  ["sigs.k8s.io/controller-tools"]="Harvesting is in progress"
  ["github.com/prometheus-operator/prometheus-operator"]="Harvesting is in progress"
)

declare -A ignored_paths_licenses=(
  # https://github.com/devfile/devworkspace-operator?tab=Apache-2.0-1-ov-file#readme
  ["github.com/devfile/devworkspace-operator"]="Apache-2.0"
  # https://github.com/go-openapi/testify?tab=Apache-2.0-1-ov-file#readme
  ["github.com/go-openapi/testify/enable/yaml/v2"]="Apache-2.0"
  # https://github.com/grpc-ecosystem/grpc-health-probe?tab=Apache-2.0-1-ov-file#readme
  ["github.com/grpc-ecosystem/grpc-health-probe"]="Apache-2.0"
  # https://github.com/miekg/pkcs11?tab=BSD-3-Clause-1-ov-file#readme
  ["github.com/miekg/pkcs11"]="BSD-3-Clause"
  # https://github.com/openshift/api?tab=Apache-2.0-1-ov-file#readme
  ["github.com/openshift/api"]="Apache-2.0"
  # https://github.com/openshift/library-go?tab=Apache-2.0-1-ov-file#readme
  ["github.com/openshift/library-go"]="Apache-2.0"
  # https://github.com/operator-framework/api/?tab=Apache-2.0-1-ov-file#readme
  ["github.com/operator-framework/api"]="Apache-2.0"
  # https://github.com/operator-framework/operator-lifecycle-manager/?tab=Apache-2.0-1-ov-file#readme
  ["github.com/operator-framework/operator-lifecycle-manager"]="Apache-2.0"
  # https://github.com/operator-framework/operator-registry/?tab=Apache-2.0-1-ov-file#readme
  ["github.com/operator-framework/operator-registry"]="Apache-2.0"
  # https://github.com/redis/go-redis?tab=BSD-2-Clause-1-ov-file#readme
  ["github.com/redis/go-redis/extra/rediscmd/v9"]="BSD-2-Clause"
  # https://github.com/redis/go-redis?tab=BSD-2-Clause-1-ov-file#readme
  ["github.com/redis/go-redis/extra/redisotel/v9"]="BSD-2-Clause"
  # https://github.com/sigstore/fulcio/tree/v1.8.5?tab=License-1-ov-file
  ["github.com/sigstore/fulcio"]="BSD-2-Clause"
  # https://github.com/open-telemetry/opentelemetry-go-contrib?tab=Apache-2.0-1-ov-file
  ["github.com/open-telemetry/opentelemetry-go-contrib"]="Apache-2.0"
  # https://github.com/open-telemetry/opentelemetry-go?tab=Apache-2.0-1-ov-file
  ["github.com/open-telemetry/opentelemetry-go"]="Apache-2.0"
  # https://github.com/containers/container-libs/?tab=readme-ov-file#license
  ["go.podman.io/common"]="Apache-2.0"
  ["go.podman.io/image/v5"]="Apache-2.0"
  ["go.podman.io/storage"]="Apache-2.0"
  # https://github.com/golang/tools/tree/go/packages/packagestest/v0.1.1-deprecated?tab=License-1-ov-file
  ["golang.org/x/tools/go/packages/packagestest"]="BSD-3-Clause"
  # https://github.com/googleapis/go-genproto?tab=Apache-2.0-1-ov-file
  ["google.golang.org/genproto"]="Apache-2.0"
  # https://github.com/kubernetes/apiextensions-apiserver?tab=Apache-2.0-1-ov-file
  ["k8s.io/apiextensions-apiserver"]="Apache-2.0"
  # https://github.com/kubernetes/apiserver?tab=Apache-2.0-1-ov-file
  ["k8s.io/apiserver"]="Apache-2.0"
  # https://github.com/kubernetes/code-generator?tab=Apache-2.0-1-ov-file
  ["k8s.io/code-generator"]="Apache-2.0"
  # https://github.com/kubernetes/component-base?tab=Apache-2.0-1-ov-file
  ["k8s.io/component-base"]="Apache-2.0"
  # https://github.com/kubernetes/kms?tab=Apache-2.0-1-ov-file
  ["k8s.io/kms"]="Apache-2.0"
  # https://github.com/kubernetes/kube-aggregator?tab=Apache-2.0-1-ov-file
  ["k8s.io/kube-aggregator"]="Apache-2.0"
  # https://github.com/kubernetes/kube-openapi?tab=Apache-2.0-1-ov-file
  ["k8s.io/kube-openapi"]="Apache-2.0"
  # https://github.com/kubernetes-sigs/controller-tools?tab=Apache-2.0-1-ov-file
  ["sigs.k8s.io/controller-tools"]="Apache-2.0"
  # https://github.com/prometheus-operator/prometheus-operator?tab=Apache-2.0-1-ov-file
  ["github.com/prometheus-operator/prometheus-operator"]="Apache-2.0"
)

declare -A declared_licenses=(
  # https://github.com/microsoft/hcsshim?tab=MIT-1-ov-file#readme
  ["github.com/Microsoft/hcsshim"]="MIT"
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
      license="${ignored_paths_licenses["$path"]}"
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

    if [[ -v declared_licenses["$path"] ]]; then
      license="${declared_licenses["$path"]}"
      license_approved=true
    else
      # analyze the license
      license=$(curl -s "$url" | jq -r '.licensed.declared')
      license="${license%% AND*}"

      # Handle OR licenses - split and check each one
      IFS=' OR ' read -ra license_parts <<< "$license"
      license_approved=false
      for license_part in "${license_parts[@]}"; do
        for allowed_license in "${allowed_licenses[@]}"; do
          if [[ "${allowed_license^^}" == "${license_part^^}" ]]; then
            license_approved=true
            break 2
          fi
        done
      done
    fi

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
