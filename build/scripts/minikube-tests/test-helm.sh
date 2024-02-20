#!/usr/bin/env bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
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

source "${OPERATOR_REPO}/build/scripts/minikube-tests/common.sh"

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

runTest() {
  # Deploy Eclipse Che to have Cert Manager and Dex installed
  chectl server:deploy \
    --batch \
    --platform minikube \
    --k8spodwaittimeout=6000000 \
    --k8spodreadytimeout=6000000 \
    --che-operator-cr-patch-yaml "${OPERATOR_REPO}/build/scripts/minikube-tests/minikube-checluster-patch.yaml"

  # Read OIDC configuration
  local IDENTITY_PROVIDER_URL=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.identityProviderURL}')
  local OAUTH_SECRET=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.oAuthSecret}')
  local OAUTH_CLIENT_NAME=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.auth.oAuthClientName}')
  local DOMAIN=$(kubectl get checluster/eclipse-che -n ${NAMESPACE} -o jsonpath='{.spec.networking.domain}')

  # Delete Eclipse Che (Cert Manager, Dev Workspace and Dex are still there)
  chectl server:delete --batch -n ${NAMESPACE}
  sleep 30s

  # Prepare HelmCharts
  HELMCHART_DIR=/tmp/chectl-helmcharts
  rm -rf "${HELMCHART_DIR}"
  cp -r "${OPERATOR_REPO}/helmcharts/next" "${HELMCHART_DIR}"

  # Set custom image
  OPERATOR_DEPLOYMENT="${HELMCHART_DIR}"/templates/che-operator.Deployment.yaml
  buildAndCopyCheOperatorImageToMinikube
  yq -riSY '.spec.template.spec.containers[0].image = "'${OPERATOR_IMAGE}'"' "${OPERATOR_DEPLOYMENT}"
  yq -riSY '.spec.template.spec.containers[0].imagePullPolicy = "Never"' "${OPERATOR_DEPLOYMENT}"

  # Patch CheCluster CR to limit resources (see minikube-checluster-patch.yaml)
  CHECLUSTER_CR="${HELMCHART_DIR}"/templates/org_v2_checluster.yaml
  yq -riY '.spec.components = null' ${CHECLUSTER_CR}
  yq -riY '.spec.components.pluginRegistry.openVSXURL = "https://open-vsx.org"' ${CHECLUSTER_CR}
  for component in pluginRegistry devfileRegistry dashboard; do
    yq -riY '.spec.components.'${component}'.deployment.containers[0].resources = {limits: {cpu: "50m"}, request: {cpu: "50m"}}' ${CHECLUSTER_CR}
  done
  yq -riY '.spec.components.cheServer.deployment.containers[0].resources.limits.cpu = "500m"' ${CHECLUSTER_CR}
  gatewayComponent=(kube-rbac-proxy oauth-proxy configbump gateway)
  for i in {0..3}; do
    yq -riY '.spec.networking.auth.gateway.deployment.containers['$i'] = {name: "'${gatewayComponent[$i]}'", resources: {limits: {cpu: "50m"}, request: {cpu: "50m"}}}' ${CHECLUSTER_CR}
  done

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

  # Free up some cpu resources
  kubectl scale deployment che --replicas=0 -n eclipse-che

  createDevWorkspace
  startAndWaitDevWorkspace
  stopAndWaitDevWorkspace
  deleteDevWorkspace
}

initDefaults
runTest