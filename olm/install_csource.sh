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

set -e -x

#TODO Create CatalogSource for kubernetes
Channel=$1
if [ -z "${Channel}" ]
  then
    echo "[ERROR] Please run the script like install_csource.sh [<channel>] [<namespace>]"
    exit 1
fi
Namespace=$2
if [ -z "${Namespace}" ]
  then
    echo "[ERROR] Please run the script like install_csource.sh [<channel>] [<namespace>]"
    exit 1
fi

BASE_DIR=$(cd "$(dirname "$0")" && pwd)

init() {
  QUAY_PROJECT=catalogsource
  CATALOG_IMAGENAME="quay.io/${QUAY_USERNAME}/${QUAY_PROJECT}"
}

check_oc_cluster() {
  # oc installed
  if ! which oc; then
    printf "[ERROR] oc client is required, please install oc client in your PATH."
    exit 1
  fi

  # oc logged in
  if ! oc whoami; then
    printf "[ERROR] Please login as a cluster-admin."
    exit 1
  fi
}

create_csource_image() {
  docker build -t ${CATALOG_IMAGENAME} -f ${BASE_DIR}/eclipse-che-preview-openshift/Dockerfile \
        ${BASE_DIR}/eclipse-che-preview-openshift
  docker login -u ${QUAY_USERNAME} -p ${QUAY_PASWORD} quay.io
  docker push ${CATALOG_IMAGENAME}
}

install_che_csource() {
  oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${Namespace}
---
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: ${Namespace}
spec:
  targetNamespaces:
  - ${Namespace}
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: che-catalogsource
  namespace: ${Namespace}
spec:
  sourceType: grpc
  image: ${CATALOG_IMAGENAME}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: che-subscription
  namespace: ${Namespace} 
spec:
  channel: ${Channel}
  installPlanApproval: Automatic
  name: eclipse-che-preview-openshift
  source: che-catalogsource
  sourceNamespace: ${Namespace} 
EOF
}

check_plan() {
set +x
i=1
while [ $i -le 720 ]
do
  installPlan=$(kubectl get subscription/"che-subscription" -n "${Namespace}" -o jsonpath='{.status.installplan.name}')
  if [ ! -z "$installPlan" ]
  then
      kubectl wait installplan/"${installPlan}" -n "${Namespace}" --for=condition=Installed --timeout=240s
      echo "[INFO] Che operator install complete"
      break
  fi
	(( i++ ))
done
  if [ $i -gt 720 ]
  then
    echo "[ERROR] Che operator install did't start"
    exit 1
  fi
}

init
check_oc_cluster
create_csource_image
install_che_csource
check_plan
