#!/usr/bin/env bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
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
set -x

# Get absolute path for root repo directory from github actions context: https://docs.github.com/en/free-pro-team@latest/actions/reference/context-and-expression-syntax-for-github-actions
export OPERATOR_REPO="${GITHUB_WORKSPACE}"
if [ -z "${OPERATOR_REPO}" ]; then
  OPERATOR_REPO=$(dirname "$(dirname "$(dirname "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")")")")
fi

source "${OPERATOR_REPO}/build/scripts/common.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  # Deploy Eclipse Che to have Cert Manager and Dex installed
  chectl server:deploy --batch --platform minikube

  # Read OIDC configuration
  local IDENTITY_PROVIDER_URL=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.identityProviderURL}')
  local OAUTH_SECRET=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.oAuthSecret}')
  local OAUTH_CLIENT_NAME=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.oAuthClientName}')
  local DOMAIN=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.domain}')

  # Delete Eclipse Che (Cert Manager and Dex are still there)
  chectl server:delete -y --delete-all -n ${NAMESPACE}
  sleep 30s

  # Prepare HelmCharts
  HELMCHART_DIR=/tmp/chectl-helmcharts
  OPERATOR_DEPLOYMENT="${HELMCHART_DIR}"/templates/che-operator.Deployment.yaml
  rm -rf "${HELMCHART_DIR}"
  cp -r "${OPERATOR_REPO}/helmcharts/next" "${HELMCHART_DIR}"

  buildAndCopyCheOperatorImageToMinikube
  yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${OPERATOR_DEPLOYMENT}"
  yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "Never"' "${OPERATOR_DEPLOYMENT}"

  # Deploy Eclipse Che with Helm
  pushd "${HELMCHART_DIR}"
  helm install che \
    --create-namespace \
    --namespace eclipse-che \
    --set networking.domain="${DOMAIN}" \
    --set networking.auth.oAuthSecret="${OAUTH_SECRET}" \
    --set networking.auth.oAuthClientName="${OAUTH_CLIENT_NAME}" \
    --set networking.auth.identityProviderURL="${IDENTITY_PROVIDER_URL}" .
  popd

  pushd ${OPERATOR_REPO}
    make wait-eclipseche-version VERSION="next" NAMESPACE=${NAMESPACE}
    make wait-devworkspace-running NAMESPACE="devworkspace-controller"
  popd
}

initDefaults
runTest