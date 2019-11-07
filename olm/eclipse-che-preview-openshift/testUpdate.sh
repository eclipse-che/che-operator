#!/bin/bash
#
# Copyright (c) 2012-2018 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

BASE_DIR=$(cd "$(dirname "$0")" && pwd)

source ${BASE_DIR}/../check-yq.sh

kubectl apply -f ${BASE_DIR}/operator-source.yaml
channel=$1
if [ "${channel}" == "" ]; then
  channel="nightly"
fi

platform="openshift"
packageName=eclipse-che-preview-${platform}
packageFolderPath="${BASE_DIR}/deploy/olm-catalog/${packageName}"
packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

lastCSV=$(yq -r ".channels[] | select(.name == \"${channel}\") | .currentCSV" "${packageFilePath}")
lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")
previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")

echo "lastPackageVersion=${lastPackageVersion}"
echo "lastCSV=${lastCSV}"
echo "previousCSV=${previousCSV}"

if kubectl get namespace eclipse-che-preview-test  >/dev/null 2>&1
then
  echo "You should delete namespace 'eclipse-che-preview-test' before running the update test first."
  exit 1
fi

echo "Installing test pre-requisistes"

kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: eclipse-che-preview-test
---
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: eclipse-che-preview-test
spec:
  targetNamespaces:
  - eclipse-che-preview-test
EOF

echo "Subscribing to previous version: ${previousCSV}"

kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: eclipse-che-preview-openshift
  namespace: eclipse-che-preview-test
spec:
  channel: ${channel}
  installPlanApproval: Manual
  name: eclipse-che-preview-openshift
  source: eclipse-che-preview-openshift
  sourceNamespace: openshift-marketplace
  startingCSV: ${previousCSV}
EOF


kubectl wait subscription/eclipse-che-preview-openshift -n eclipse-che-preview-test --for=condition=InstallPlanPending --timeout=240s

installPlan=$(kubectl get subscription/eclipse-che-preview-openshift -n eclipse-che-preview-test -o jsonpath='{.status.installplan.name}')

kubectl patch installplan/"${installPlan}" -n eclipse-che-preview-test --type=merge -p '{"spec":{"approved":true}}'

echo "Creating Custom Resource"

CRs=$(yq -r '.metadata.annotations["alm-examples"]' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
CR=$(echo "$CRs" | yq -r ".[0]")

echo "$CR" | kubectl apply -n eclipse-che-preview-test -f -

echo "Waiting for Che server to be deployed"

i=0
while [ $i -le 240 ]
do
  status=$(kubectl get checluster/eclipse-che -n eclipse-che-preview-test -o jsonpath={.status.cheClusterRunning})
  if [ "${status}" == "Available" ]
  then
    break
  fi
  sleep 1
  ((i++))
done

if [ $i -gt 240 ]
then
  echo "Che server did't start after 4 minutes"
  exit 1
fi

installPlan=$(kubectl get subscription/eclipse-che-preview-openshift -n eclipse-che-preview-test -o jsonpath='{.status.installplan.name}')

kubectl patch installplan/"${installPlan}" -n eclipse-che-preview-test --type=merge -p '{"spec":{"approved":true}}'
