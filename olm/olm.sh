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

CATALOG_BUNDLE_IMAGE_NAME_LOCAL="localhost:5000/che_operator_bundle:0.0.1"
CATALOG_IMAGENAME="localhost:5000/testing_catalog:0.0.1"
packageName=eclipse-che-preview-${platform}
platformPath=${BASE_DIR}/${packageName}
packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
# Todo check, maybe it's unused...
packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
CSV="eclipse-che-preview-${platform}.${PACKAGE_VERSION}"

echo -e "\u001b[32m PACKAGE_VERSION=${PACKAGE_VERSION} \u001b[0m"
echo -e "\u001b[32m CSV=${CSV} \u001b[0m"
echo -e "\u001b[32m Channel=${channel} \u001b[0m"
echo -e "\u001b[32m Namespace=${namespace} \u001b[0m"

# We don't need to delete ${namespace} anymore since tls secret is precreated there.
# if kubectl get namespace "${namespace}" >/dev/null 2>&1
# then
#   echo "You should delete namespace '${namespace}' before running the update test first."
#   exit 1
# fi

catalog_source() {
  echo "--- Use default eclipse che application registry ---"
  if [ ${SOURCE_INSTALL} == "LocalCatalog" ]; then
    marketplaceNamespace=${namespace};
    kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${packageName}
  namespace: ${namespace}
spec:
  sourceType: grpc
  image: ${CATALOG_IMAGENAME}
EOF
  else
    cat ${platformPath}/operator-source.yaml
    kubectl apply -f ${platformPath}/operator-source.yaml
  fi
}

# do it only when it's required or remove using deprecated operator source
applyCheOperatorSource() {
  # echo "Apply che-operator source"
  if [ "${APPLICATION_REGISTRY}" == "" ]; then
    catalog_source
  else
    echo "---- Use non default application registry ${APPLICATION_REGISTRY} ---"

    cat "${platformPath}/operator-source.yaml" | \
    sed  -e "s/registryNamespace:.*$/registryNamespace: \"${APPLICATION_REGISTRY}\"/" | \
    kubectl apply -f -
  fi
}

# rename enable local image registry...
enableDockerRegistry() {
  if [ "${platform}" == "kubernetes" ]; then
    # check if registry addon is enabled....
    minikube addons enable registry
    minikube addons enable ingress

    docker rm -f "$(docker ps -aq --filter "name=minikube-socat")" || true
    docker run --detach --rm --name="minikube-socat" --network=host alpine ash -c "apk add socat && socat TCP-LISTEN:5000,reuseaddr,fork TCP:$(minikube ip):5000"
  
    sleep 5
  fi
}

build_Bundle_Image() {
  OPM_BUNDLE_DIR="eclipse-che-preview-${platform}/deploy/olm-catalog/eclipse-che-preview-${platform}/${PACKAGE_VERSION}/manifests"

  echo "[INFO] build bundle image for dir: ${OPM_BUNDLE_DIR}"

  ${OPM_BINARY} alpha bundle build \
    -d "${OPM_BUNDLE_DIR}" \
    --tag "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
    --package "eclipse-che-preview-${platform}" \
    --channels "stable,nightly" \
    --default "stable" \
    --image-builder docker

  opm alpha bundle validate -t "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" 

  docker push "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"
}

build_Catalog_Image() {
  if [ "${platform}" == "kubernetes" ]; then
    ${OPM_BINARY} index add \
      --bundles "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \
      --tag ${CATALOG_IMAGENAME} \
      --build-tool docker \
      --skip-tls # local registry launched without https
      # --from-index  "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" \

    docker push ${CATALOG_IMAGENAME}
    # docker save ${CATALOG_IMAGENAME} > /tmp/catalog.tar

    # docker tag "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" "${CATALOG_BUNDLE_IMAGE_NAME}"
    # docker save "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" > /tmp/bundle.tar
    # docker push "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

    # eval "$(minikube docker-env)"

    # move to clean up section...
    # docker load -i /tmp/catalog.tar && rm -rf /tmp/catalog.tar
    # docker load -i /tmp/bundle.tar && rm -rf /tmp/bundle.tar
  fi
}

# docker_build() {
#   docker build -t ${CATALOG_IMAGENAME} -f "${BASE_DIR}"/eclipse-che-preview-"${platform}"/Dockerfile \
#     "${BASE_DIR}"/eclipse-che-preview-"${platform}"
# }

# build_Catalog_Image() {
  # if [ "${platform}" == "kubernetes" ]; then
    # eval "$(minikube docker-env)"
    # docker_build
    # It should not be here...
    # minikube addons enable ingress
  # else
  #   docker_build
  #   curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.14.1/install.sh | bash -s 0.14.1
  #   docker save ${CATALOG_IMAGENAME} > /tmp/catalog.tar
  #   eval "$(minishift docker-env)"
  #   docker load -i /tmp/catalog.tar && rm -rf /tmp/catalog.tar
  # fi
# }

installOPM() {
  OPM_TEMP_DIR="$(mktemp -q -d -t "OPM_XXXXXX" 2>/dev/null || mktemp -q -d)"
  pushd "${OPM_TEMP_DIR}" || exit

  OPM_BINARY=$(command -v opm)
  if [[ ! -x $OPM_BINARY ]]; then
    echo "----Download 'opm' cli tool...----"
    curl -sLo opm "$(curl -sL https://api.github.com/repos/operator-framework/operator-registry/releases/latest | jq -r '[.assets[] | select(.name == "linux-amd64-opm")] | first | .browser_download_url')"
    export OPM_BINARY="${OPM_TEMP_DIR}/opm"
    chmod +x "${OPM_BINARY}"
    echo "----Downloading completed!----"
  fi
  echo "[INFO] 'opm' binary path: ${OPM_BINARY}"

  popd || exit
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
    # curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.15.1/install.sh -o install.sh
    # chmod +x install.sh
    # ./install.sh 0.15.1
    # rm -rf install.sh

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
EOF

# startingCSV: ${CSV}

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
  echo "[INFO] Install operator package ${packageName} into namespace ${namespace}"
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

  CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/${PACKAGE_VERSION}/manifests/${packageName}.${PACKAGE_VERSION}.clusterserviceversion.yaml")
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
  while [ $i -le 480 ]
  do
    status=$(kubectl get checluster/eclipse-che -n "${namespace}" -o jsonpath={.status.cheClusterRunning})
    if [ "${status}" == "Available" ]
    then
      break
    fi
    sleep 1
    ((i++))
  done

  if [ $i -gt 480 ]
  then
    echo "Che server did't start after 8 minutes"
    exit 1
  fi
}
