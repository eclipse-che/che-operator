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

set -ex

trap 'Catch_Finish $?' EXIT SIGINT

# Catch errors and force to delete minishift VM.
Catch_Finish() {
  rm -rf ${OPERATOR_REPO}/tmp ~/.minishift && yes | minishift delete
}

init() {
  export SCRIPT=$(readlink -f "$0")
  export SCRIPT_DIR=$(dirname "$SCRIPT")
  export RAM_MEMORY=8192
  export PLATFORM="openshift"
  export NAMESPACE="che"
  export CHANNEL="stable"

  if [[ ${WORKSPACE} ]] && [[ -d ${WORKSPACE} ]]; then
    OPERATOR_REPO=${WORKSPACE};
  else
    OPERATOR_REPO=$(dirname "$SCRIPT_DIR");
  fi

  # Create tmp folder and add che operator templates used by server:update command.
  mkdir -p "$OPERATOR_REPO/tmp" && chmod 777 "$OPERATOR_REPO/tmp"
  cp -r deploy "$OPERATOR_REPO/tmp/che-operator"
}

installDependencies() {
  installYQ
  installJQ
  install_VirtPackages
  installStartDocker
  start_libvirt
  setup_kvm_machine_driver
  minishift_installation
  installChectl
  load_jenkins_vars
}

installLatestCheStable() {
  # Get Stable and new release versions from olm files openshift.
  export packageName=eclipse-che-preview-${PLATFORM}
  export platformPath=${OPERATOR_REPO}/olm/${packageName}
  export packageFolderPath="${platformPath}/deploy/olm-catalog/${packageName}"
  export packageFilePath="${packageFolderPath}/${packageName}.package.yaml"

  export lastCSV=$(yq -r ".channels[] | select(.name == \"${CHANNEL}\") | .currentCSV" "${packageFilePath}")
  export lastPackageVersion=$(echo "${lastCSV}" | sed -e "s/${packageName}.v//")
  export previousCSV=$(sed -n 's|^ *replaces: *\([^ ]*\) *|\1|p' "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml")
  export previousPackageVersion=$(echo "${previousCSV}" | sed -e "s/${packageName}.v//")

  # Add stable Che images and tag to CR
  sed -i "s/cheImage: ''/cheImage: quay.io\/eclipse\/che-server/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml
  sed -i "s/cheImageTag: ''/cheImageTag: ${previousPackageVersion}/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml
  sed -i "s/devfileRegistryImage: ''/devfileRegistryImage: quay.io\/eclipse\/che-devfile-registry:"${previousPackageVersion}"/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml
  sed -i "s/pluginRegistryImage: ''/pluginRegistryImage: quay.io\/eclipse\/che-plugin-registry:"${previousPackageVersion}"/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml
  sed -i "s/identityProviderImage: ''/identityProviderImage: quay.io\/eclipse\/che-keycloak:"${previousPackageVersion}"/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml

  # set 'openShiftoAuth: false'
  sed -i "s/openShiftoAuth: .*/openShiftoAuth: false/" ${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml

  # Start last stable version of che
  chectl server:start --platform=minishift --skip-kubernetes-health-check --che-operator-cr-yaml=${OPERATOR_REPO}/tmp/che-operator/crds/org_v1_che_cr.yaml \
    --che-operator-image=quay.io/eclipse/che-operator:${previousPackageVersion} --installer=operator
}

# Utility to wait for new release to be up
waitForNewCheVersion() {
  export n=0

  while [ $n -le 500 ]
  do
    cheVersion=$(oc get checluster/eclipse-che -n "${NAMESPACE}" -o jsonpath={.status.cheVersion})
    oc get pods -n ${NAMESPACE}
    if [ "${cheVersion}" == $lastPackageVersion ]
    then
      echo -e "\u001b[32m Installed latest version che-operator: ${lastCSV} \u001b[0m"
      break
    fi
    sleep 6
    n=$(( n+1 ))
  done

  if [ $n -gt 360 ]
  then
    echo "Latest version install for Eclipse che failed."
    exit 1
  fi
}

self_signed_minishift() {
  export DOMAIN=*.$(minishift ip).nip.io

  source ${OPERATOR_REPO}/.ci/util/che-cert-generation.sh

  #Configure Router with generated certificate:

  oc login -u system:admin --insecure-skip-tls-verify=true
  oc project default
  oc delete secret router-certs

  cat domain.crt domain.key > minishift.crt
  oc create secret tls router-certs --key=domain.key --cert=minishift.crt
  oc rollout latest router

  oc create namespace che

  cp rootCA.crt ca.crt
  oc create secret generic self-signed-certificate --from-file=ca.crt -n=che
}

testUpdates() {
  # Install previous stable version of Eclipse Che
  self_signed_minishift
  installLatestCheStable

  # Create an workspace
  getCheAcessToken # Function from ./util/ci_common.sh
  chectl workspace:create --devfile=$OPERATOR_REPO/.ci/util/devfile-test.yaml

  # Update the operator to the new release
  chectl server:update --skip-version-check --installer=operator --platform=minishift --che-operator-image=quay.io/eclipse/che-operator:${lastPackageVersion} --templates="tmp"

  # Patch images and tag the latest release
  oc patch checluster eclipse-che --type='json' -p='[{"op": "replace", "path": "/spec/auth/identityProviderImage", "value":"quay.io/eclipse/che-keycloak:'${lastPackageVersion}'"}]' -n ${NAMESPACE}
  oc patch checluster eclipse-che --type='json' -p='[{"op": "replace", "path": "/spec/server/devfileRegistryImage", "value":"quay.io/eclipse/che-devfile-registry:'${lastPackageVersion}'"}]' -n ${NAMESPACE}
  oc patch checluster eclipse-che --type='json' -p='[{"op": "replace", "path": "/spec/server/pluginRegistryImage", "value":"quay.io/eclipse/che-plugin-registry:'${lastPackageVersion}'"}]' -n ${NAMESPACE}
  oc patch checluster eclipse-che --type='json' -p='[{"op": "replace", "path": "/spec/server/cheImageTag", "value":"'${lastPackageVersion}'"}]' -n ${NAMESPACE}
  waitForNewCheVersion

  getCheAcessToken # Function from ./util/ci_common.sh
  workspaceList=$(chectl workspace:list)
  workspaceID=$(echo "$workspaceList" | grep -oP '\bworkspace.*?\b')
  chectl workspace:start $workspaceID

  # Wait for workspace to be up
  waitWorkspaceStart  # Function from ./util/ci_common.sh
  oc get events -n ${NAMESPACE}
}

init
source "${OPERATOR_REPO}"/.ci/util/ci_common.sh
installDependencies
testUpdates
