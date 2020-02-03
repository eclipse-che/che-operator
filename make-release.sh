#!/bin/bash
#
# Copyright (c) 2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

init() {
  RED='\e[31m'
  NC='\e[0m'
  YELLOW='\e[33m'
  GREEN='\e[32m'

  RELEASE="$1"
  GIT_REMOTE_UPSTREAM="git@github.com:eclipse/che-operator.git"
  CURRENT_DIR=$(pwd)
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
}

check() {
  echo -e $RED"##############################################"
  echo -e $RED"  This is a draft version of release script."
  echo -e $RED"  It is needed to check all steps manually."
  echo -e $RED"##############################################"$NC


  if [ $# -ne 1 ]; then
    printf "%bError: %bWrong number of parameters.\nUsage: ./make-release.sh <version>\n" "${RED}" "${NC}"
    exit 1
  fi

  [ -z "$QUAY_USERNAME" ] && echo -e $RED"QUAY_USERNAME is not set"$NC && exit 1
  [ -z "$QUAY_PASSWORD" ] && echo -e $RED"QUAY_PASSWORD is not set"$NC && exit 1
  command -v operator-courier >/dev/null 2>&1 || { echo -e $RED"operator-courier is not installed. Aborting."$NC; exit 1; }
  command -v operator-sdk >/dev/null 2>&1 || { echo -e $RED"operator-sdk is not installed. Aborting."$NC; exit 1; }

  local operatorVersion=$(operator-sdk version)
  [[ $operatorVersion =~ .*v0.10.0.* ]] || { echo -e $RED"operator-sdk v0.10.0 is required"$NC; exit 1; }
}

ask() {
  while true; do
    echo -e -n $GREEN$@$NC" (Y)es or (N)o "
    read -r yn
    case $yn in
      [Yy]* ) return 0;;
      [Nn]* ) return 1;;
      * ) echo "Please answer (Y)es or (N)o. ";;
    esac
  done
}

resetLocalChanges() {
  set +e
  ask "1. Reset local changes?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    git fetch ${GIT_REMOTE_UPSTREAM}
    git pull ${GIT_REMOTE_UPSTREAM} master
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

createLocalBranch() {
  set +e
  ask "2. Create local '$RELEASE' branch?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    git checkout -b $RELEASE
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

getPropertyValue() {
  local file=$1
  local key=$2
  echo $(cat $file | grep -m1 "$key" | tr -d ' ' | tr -d '\t' | cut -d = -f2)
}

releaseOperatorCode() {
  set +e
  ask "3. Release operator code?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    local defaultsgo=$BASE_DIR/pkg/deploy/defaults.go
    local extraimagesgo=$BASE_DIR/pkg/deploy/extra_images.go

    echo -e $GREEN"3.1 Launch 'release-operator-code.sh' script"$NC
    . ${BASE_DIR}/release-operator-code.sh $RELEASE

    echo -e $GREEN"3.2 Validate changes for $defaultsgo"$NC
    [[ \"$RELEASE\" != $(getPropertyValue $defaultsgo defaultCheServerImageTag) ]] && { echo -e $RED"$defaultsgo cotains unexpected changes"$NC; exit 1; }
    [[ \"quay.io/eclipse/che-devfile-registry:$RELEASE\" != $(getPropertyValue $defaultsgo defaultDevfileRegistryUpstreamImage) ]] && { echo -e $RED"$defaultsgo cotains unexpected changes"$NC; exit 1; }
    [[ \"quay.io/eclipse/che-plugin-registry:$RELEASE\" != $(getPropertyValue $defaultsgo defaultPluginRegistryUpstreamImage) ]] && { echo -e $RED"$defaultsgo cotains unexpected changes"$NC; exit 1; }
    [[ \"quay.io/eclipse/che-keycloak:$RELEASE\" != $(getPropertyValue $defaultsgo defaultKeycloakUpstreamImage) ]] && { echo -e $RED"$defaultsgo cotains unexpected changes"$NC; exit 1; }

    echo -e $GREEN"3.3 Validate changes for $extraimagesgo"$NC
    [[ \"\" == $(getPropertyValue $extraimagesgo defaultCheWorkspacePluginBrokerMetadataUpstreamImage) ]] && { echo $RED"$extraimagesgo cotains unexpected changes"$NC; exit 1; }
    [[ \"\" == $(getPropertyValue $extraimagesgo defaultCheWorkspacePluginBrokerArtifactsUpstreamImage) ]] && { echo $RED"$extraimagesgo cotains unexpected changes"$NC; exit 1; }
    [[ \"\" == $(getPropertyValue $extraimagesgo defaultCheServerSecureExposerJwtProxyUpstreamImage) ]] && { echo $RED"$extraimagesgo cotains unexpected changes"$NC; exit 1; }

    echo -e $GREEN"3.4 Validate number of changed files"$NC
    local changes=$(git status -s | wc -l)
    [[ $changes -gt 2 ]] && { echo -e $RED"The number of changes are greated then 2. Check 'git status'."$NC; return 1; }
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

commitDefaultsGoChanges() {
  set +e
  ask "4. Commit changes?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    git commit -am "Update defaults tags to "$RELEASE --signoff
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

pushImage() {
  set +e
  ask "5. Push image to quay.io?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    docker login quay.io
    docker push quay.io/eclipse/che-operator:$RELEASE
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

releaseOlmFiles() {
  set +e
  ask "6. Release OLM files?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    echo -e $GREEN"6.1 Launch 'olm/release-olm-files.sh' script"$NC
    cd $BASE_DIR/olm
    . $BASE_DIR/olm/release-olm-files.sh $RELEASE
    cd $CURRENT_DIR

    local openshift=$BASE_DIR/eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift
    local kubernetes=$BASE_DIR/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes

    echo -e $GREEN"6.2 Validate files"$NC
    grep -q "currentCSV: eclipse-che-preview-openshift.v"$RELEASE $openshift/eclipse-che-preview-openshift.package.yaml
    grep -q "currentCSV: eclipse-che-preview-kubernetes.v"$RELEASE $kubernetes/eclipse-che-preview-kubernetes.package.yaml
    grep -q "version: "$RELEASE $openshift/$RELEASE/eclipse-che-preview-openshift.v$RELEASE.clusterserviceversion.yaml
    grep -q "version: "$RELEASE $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.v$RELEASE.clusterserviceversion.yaml
    test -f $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.crd.yaml
    test -f $openshift/$RELEASE/eclipse-che-preview-openshift.crd.yaml

    echo -e $GREEN"6.3 It is needed to check diff files manully"$NC
    echo $openshift/$RELEASE/eclipse-che-preview-openshift.v$RELEASE.clusterserviceversion.yaml.diff
    echo $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.v$RELEASE.clusterserviceversion.yaml.diff
    echo $openshift/$RELEASE/eclipse-che-preview-openshift.crd.yaml.diff
    echo $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.crd.yaml.diff
    read -p "Press enter to continue"

    echo -e $GREEN"6.4 Validate number of changed files"$NC
    local changes=$(git status -s | wc -l)
    [[ $changes -gt 4 ]] && { echo -e $RED"The number of changed files are greated then 4. Check 'git status'."$NC; return 1; }
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

commitOlmChanges() {
  set +e
  ask "7. Commit changes?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    git add -A
    git commit -m "Release OLM files to "$RELEASE --signoff
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

pushOlmFiles() {
  set +e
  ask "8. Push OLM files to quay.io?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    cd $BASE_DIR/olm
    . $BASE_DIR/olm/push-olm-files-to-quay.sh
    cd $CURRENT_DIR

    read -p "Validate RELEASES page on quay.io. Press enter to open the browser"
    xdg-open https://quay.io/application/eclipse-che-operator-kubernetes/eclipse-che-preview-kubernetes

    read -p "Validate RELEASES page on quay.io. Press enter to open the browser"
    xdg-open https://quay.io/application/eclipse-che-operator-openshift/eclipse-che-preview-openshift

    read -p "Press enter to continue"
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

pushChanges() {
  set +e
  ask "9. Push changes?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    git push origin $RELEASE
    git tag -a $RELEASE -m $RELEASE
    git push --tags origin
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

run() {
  resetLocalChanges
  createLocalBranch
  releaseOperatorCode
  commitDefaultsGoChanges
  pushImage
  releaseOlmFiles
  commitOlmChanges
  pushOlmFiles
  pushChanges
}

init "$@"
check "$@"
run "$@"
