#!/bin/bash
#
# Copyright (c) 2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#
# Scripts to prepare OLM(operator lifecycle manager) and install che-operator package
# with specific version using OLM.

BASE_DIR=$(cd "$(dirname "$0")" && pwd)

source ${BASE_DIR}/check-yq.sh

platform=$1
if [ "${platform}" == "" ]; then
  echo "Please specify platform ('openshift' or 'kubernetes') as the first argument."
  echo ""
  echo "testUpdate.sh <platform> [<channel>] [<namespace>]"
  exit 1
fi

PACKAGE_VERSION=$2
if [ "${PACKAGE_VERSION}" == "" ]; then
  echo "Please specify PACKAGE_VERSION version"
  exit 1
fi

namespace=$3
if [ "${namespace}" == "" ]; then
  namespace="eclipse-che-preview-test"
fi

channel="stable"
if [[ "${PACKAGE_VERSION}" =~ "nightly" ]]
then
   channel="nightly"
fi

packageName=eclipse-che-preview-${platform}
platformPath=${BASE_DIR}/${packageName}
packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
CSV="eclipse-che-preview-${platform}.v${PACKAGE_VERSION}"

echo -e "\u001b[32m PACKAGE_VERSION=${PACKAGE_VERSION} \u001b[0m"
echo -e "\u001b[32m CSV=${CSV} \u001b[0m"
echo -e "\u001b[32m Channel=${channel} \u001b[0m"
echo -e "\u001b[32m Namespace=${namespace} \u001b[0m"

if kubectl get namespace "${namespace}" >/dev/null 2>&1
then
  echo "You should delete namespace '${namespace}' before running the update test first."
  exit 1
fi

applyCheOperatorSource() {
  echo "Apply che-operator source"
  if [ "${APPLICATION_REGISTRY}" == "" ]; then
    echo "--- Use default eclipse che application registry ---"

    kubectl apply -f ${platformPath}/operator-source.yaml
  else
    echo "---- Use non default application registry ${APPLICATION_REGISTRY} ---"

    cat ${platformPath}/operator-source.yaml | \
    sed  -e "s/registryNamespace:.*$/registryNamespace: \"${APPLICATION_REGISTRY}\"/" | \
    kubectl apply -f -
  fi
 }

installOperatorMarketPlace() {
  echo "Installing test pre-requisistes"

  marketplaceNamespace="marketplace"
  if [ "${platform}" == "openshift" ]
  then
    marketplaceNamespace="openshift-marketplace"
    applyCheOperatorSource
  else
    curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/install.sh | bash -s 0.12.0
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/01_namespace.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/02_catalogsourceconfig.crd.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/03_operatorsource.crd.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/04_service_account.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/05_role.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/06_role_binding.yaml
    sleep 1
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/07_upstream_operatorsource.cr.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/08_operator.yaml

    applyCheOperatorSource

    i=0
    while [ $i -le 120 ]
    do
      if kubectl get catalogsource/"${packageName}" -n "${marketplaceNamespace}"  >/dev/null 2>&1
      then
        break
      fi
      sleep 1
      ((i++))
    done

    if [ $i -gt 120 ]
    then
      echo "Catalog source not created after 2 minutes"
      exit 1
    fi

    kubectl get catalogsource/"${packageName}" -n "${marketplaceNamespace}" -o json | jq '.metadata.namespace = "olm" | del(.metadata.creationTimestamp) | del(.metadata.uid) | del(.metadata.resourceVersion) | del(.metadata.generation) | del(.metadata.selfLink) | del(.status)' | kubectl apply -f -
    marketplaceNamespace="olm"
  fi

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
---
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: ${namespace}
spec:
  targetNamespaces:
  - ${namespace}
EOF

  echo "Subscribing to version: ${CSV}"

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  channel: ${channel}
  installPlanApproval: Manual
  name: ${packageName}
  source: ${packageName}
  sourceNamespace: ${marketplaceNamespace}
  startingCSV: ${CSV}
EOF

  kubectl describe subscription/"${packageName}" -n "${namespace}"

  kubectl wait subscription/"${packageName}" -n "${namespace}" --for=condition=InstallPlanPending --timeout=240s
  if [ $? -ne 0 ]
  then
    echo Subscription failed to install the operator
    exit 1
  fi

  kubectl describe subscription/"${packageName}" -n "${namespace}"
}

installPackage() {
  echo "Install operator package ${packageName} into namespace ${namespace}"
  installPlan=$(kubectl get subscription/"${packageName}" -n "${namespace}" -o jsonpath='{.status.installplan.name}')

  kubectl patch installplan/"${installPlan}" -n "${namespace}" --type=merge -p '{"spec":{"approved":true}}'

  kubectl wait installplan/"${installPlan}" -n "${namespace}" --for=condition=Installed --timeout=240s
  if [ $? -ne 0 ]
  then
    echo InstallPlan failed to install the operator
    exit 1
  fi
}

applyCRCheCluster() {
  echo "Creating Custom Resource"

  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/${PACKAGE_VERSION}/${packageName}.v${PACKAGE_VERSION}.clusterserviceversion.yaml")
  CR=$(echo "$CRs" | yq -r ".[0]")
  if [ "${platform}" == "kubernetes" ]
  then
    CR=$(echo "$CR" | yq -r ".spec.k8s.ingressDomain = \"$(minikube ip).nip.io\"")
  fi

  echo "$CR" | kubectl apply -n "${namespace}" -f -
}

waitCheServerDeploy() {
  echo "Waiting for Che server to be deployed"

  i=0
  while [ $i -le 360 ]
  do
    status=$(kubectl get checluster/eclipse-che -n "${namespace}" -o jsonpath={.status.cheClusterRunning})
    if [ "${status}" == "Available" ]
    then
      break
    fi
    sleep 1
    ((i++))
  done

  if [ $i -gt 360 ]
  then
    echo "Che server did't start after 6 minutes"
    exit 1
  fi
}
