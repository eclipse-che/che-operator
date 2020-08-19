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

SCRIPT=$(readlink -f "$0")
export SCRIPT
BASE_DIR=$(dirname "$(dirname "$SCRIPT")")/olm;
export BASE_DIR

source ${BASE_DIR}/check-yq.sh

SOURCE_INSTALL=$4

if [ -z ${SOURCE_INSTALL} ]; then SOURCE_INSTALL="Marketplace"; fi

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

# We don't need to delete ${namepsace} anymore since tls secret is precreated there.
# if kubectl get namespace "${namespace}" >/dev/null 2>&1
# then
#   echo "You should delete namespace '${namespace}' before running the update test first."
#   exit 1
# fi

catalog_source() {
  echo "--- Use default eclipse che application registry ---"
  if [ ${SOURCE_INSTALL} == "catalog" ]; then
    marketplaceNamespace=${namespace};
    kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  sourceType: grpc
  image: ${CATALOG_SOURCE_IMAGE}

EOF
  else
    cat ${platformPath}/operator-source.yaml
    kubectl apply -f ${platformPath}/operator-source.yaml
  fi
}

applyCheOperatorSource() {
  echo "Apply che-operator source"
  if [ "${APPLICATION_REGISTRY}" == "" ]; then
    catalog_source
  else
    echo "---- Use non default application registry ${APPLICATION_REGISTRY} ---"

    cat ${platformPath}/operator-source.yaml | \
    sed  -e "s/registryNamespace:.*$/registryNamespace: \"${APPLICATION_REGISTRY}\"/" | \
    kubectl apply -f -
  fi
 }

installOperatorMarketPlace() {
  echo "Installing test pre-requisistes"
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
EOF
  marketplaceNamespace="marketplace"
  if [ "${platform}" == "openshift" ];
  then
    marketplaceNamespace="openshift-marketplace";
    applyCheOperatorSource
  else
    OLM_VERSION=0.15.1
    MARKETPLACE_VERSION=4.5
    OPERATOR_MARKETPLACE_VERSION="release-${MARKETPLACE_VERSION}"
    curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/install.sh | bash -s ${OLM_VERSION}
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/01_namespace.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/03_operatorsource.crd.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/04_service_account.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/05_role.yaml
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/06_role_binding.yaml
    sleep 1
    kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/07_upstream_operatorsource.cr.yaml
    curl -sL https://raw.githubusercontent.com/operator-framework/operator-marketplace/${OPERATOR_MARKETPLACE_VERSION}/deploy/upstream/08_operator.yaml | \
    sed -e "s;quay.io/openshift/origin-operator-marketplace:latest;quay.io/openshift/origin-operator-marketplace:${MARKETPLACE_VERSION};" | \
    kubectl apply -f -

    applyCheOperatorSource

    i=0
    while [ $i -le 240 ]
    do
      if kubectl get catalogsource/"${packageName}" -n "${marketplaceNamespace}"  >/dev/null 2>&1
      then
        break
      fi
      sleep 1
      ((i++))
    done

    if [ $i -gt 240 ]
    then
      echo "Catalog source not created after 4 minutes"
      exit 1
    fi
    if [ "${SOURCE_INSTALL}" == "Marketplace" ]; then
      kubectl get catalogsource/"${packageName}" -n "${marketplaceNamespace}" -o json | jq '.metadata.namespace = "olm" | del(.metadata.creationTimestamp) | del(.metadata.uid) | del(.metadata.resourceVersion) | del(.metadata.generation) | del(.metadata.selfLink) | del(.status)' | kubectl apply -f -
      marketplaceNamespace="olm"
    fi
  fi

  echo "Subscribing to version: ${CSV}"

  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: ${namespace}
spec:
  targetNamespaces:
  - ${namespace}
---
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
  set +e -x

  i=0
  while [[ $i -le 480 ]]
  do
    status=$(kubectl get checluster/eclipse-che -n "${namespace}" -o jsonpath={.status.cheClusterRunning})
    if [ "${status:-UNAVAILABLE}" == "Available" ]
    then
      break
    fi
    sleep 10
    ((i++))
  done

  if [ $i -gt 480 ]
  then
    echo "Che server did't start after 8 minutes"
    exit 1
  fi
}
