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
  BRANCH=${2:-master}
  GIT_REMOTE_UPSTREAM="git@github.com:eclipse/che-operator.git"
  CURRENT_DIR=$(pwd)
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)

  local ubiMinimal8Version=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.version')
  local ubiMinimal8Release=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.release')
  UBI8_MINIMAL_IMAGE="registry.access.redhat.com/ubi8-minimal:"$ubiMinimal8Version"-"$ubiMinimal8Release
  local test=$(skopeo inspect docker://$UBI8_MINIMAL_IMAGE)
}

check() {
  if [ $# -lt 1 ]; then
    printf "%bError: %bWrong number of parameters.\nUsage: ./make-release.sh <version>\n" "${RED}" "${NC}"
    exit 1
  fi

  [ -z "$QUAY_USERNAME" ] && echo -e $RED"QUAY_USERNAME is not set"$NC && exit 1
  [ -z "$QUAY_PASSWORD" ] && echo -e $RED"QUAY_PASSWORD is not set"$NC && exit 1
  command -v operator-courier >/dev/null 2>&1 || { echo -e $RED"operator-courier is not installed. Aborting."$NC; exit 1; }
  command -v operator-sdk >/dev/null 2>&1 || { echo -e $RED"operator-sdk is not installed. Aborting."$NC; exit 1; }
  command -v skopeo >/dev/null 2>&1 || { echo -e $RED"skopeo is not installed. Aborting."$NC; exit 1; }

  local operatorVersion=$(operator-sdk version)
  [[ $operatorVersion =~ .*v0.10.0.* ]] || { echo -e $RED"operator-sdk v0.10.0 is required"$NC; exit 1; }

  echo "Release '$RELEASE' from branch '$BRANCH'"
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
    git reset --hard
    git checkout $BRANCH
    git fetch ${GIT_REMOTE_UPSTREAM} --prune
    git pull ${GIT_REMOTE_UPSTREAM} $BRANCH
    git checkout -B $RELEASE

    local changes=$(git status -s | wc -l)
    [[ $changes -gt 0 ]] && { echo -e $RED"The number of changes are greated then 0. Check 'git status'."$NC; return 1; }
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

getPropertyValue() {
  local file=$1
  local key=$2
  echo $(cat $file | grep -m1 "$key" | tr -d ' ' | tr -d '\t' | cut -d = -f2)
}

checkImageReferences() {
  local filename=$1

  if ! grep -q "value: ${RELEASE}" $filename; then
    echo -e $RED" Unable to find Che version ${RELEASE} in the $filename"$NC; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-server:$RELEASE" $filename; then
    echo -e $RED" Unable to find Che server image with version ${RELEASE} in the $filename"$NC; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-plugin-registry:$RELEASE" $filename; then
    echo -e $RED" Unable to find plugin registry image with version ${RELEASE} in the $filename"$NC; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-devfile-registry:$RELEASE" $filename; then
    echo -e $RED" Unable to find devfile registry image with version ${RELEASE} in the $filename"$NC; exit 1
  fi

  if ! grep -q "value: quay.io/eclipse/che-keycloak:$RELEASE" $filename; then
    echo -e $RED" Unable to find che-keycloak image with version ${RELEASE} in the $filename"$NC; exit 1
  fi

  if ! grep -q "value: $IMAGE_default_pvc_jobs" $filename; then
    echo -e $RED" Unable to find ubi8_minimal image in the $filename"$NC; exit 1
  fi

  wget https://raw.githubusercontent.com/eclipse/che/${RELEASE}/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties

  plugin_broker_meta_image=$(cat /tmp/che.properties | grep  che.workspace.plugin_broker.metadata.image | cut -d '=' -f2)
  if ! grep -q "value: $plugin_broker_meta_image" $filename; then
    echo -e $RED" Unable to find plugin broker meta image '$plugin_broker_meta_image' in the $filename"$NC; exit 1
  fi

  plugin_broker_artifacts_image=$(cat /tmp/che.properties | grep  che.workspace.plugin_broker.artifacts.image | cut -d '=' -f2)
  if ! grep -q "value: $plugin_broker_artifacts_image" $filename; then
    echo -e $RED" Unable to find plugin broker artifacts image '$plugin_broker_artifacts_image' in the $filename"$NC; exit 1
  fi

  jwt_proxy_image=$(cat /tmp/che.properties | grep  che.server.secure_exposer.jwtproxy.image | cut -d '=' -f2)
  if ! grep -q "value: $jwt_proxy_image" $filename; then
    echo -e $RED" Unable to find jwt proxy image $jwt_proxy_image in the $filename"$NC; exit 1
  fi
}

releaseOperatorCode() {
  set +e
  ask "2. Release operator code?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then

    echo -e $GREEN"2.1 Launch 'release-operator-code.sh' script"$NC
    . ${BASE_DIR}/release-operator-code.sh $RELEASE $UBI8_MINIMAL_IMAGE

    local operatoryaml=$BASE_DIR/deploy/operator.yaml
    echo -e $GREEN"2.2 Validate changes for $operatoryaml"$NC
    checkImageReferences $operatoryaml

    local operatorlocalyaml=$BASE_DIR/deploy/operator-local.yaml
    echo -e $GREEN"2.2 Validate changes for $operatorlocalyaml"$NC
    checkImageReferences $operatorlocalyaml

    echo -e $GREEN"2.4 It is needed to check files manully:"$NC
    echo $operatoryaml
    echo $operatorlocalyaml
    read -p "Press enter to continue"

    echo -e $GREEN"2.6 Validate number of changed files"$NC
    local changes=$(git status -s | wc -l)
    [[ $changes -gt 1 ]] && { echo -e $RED"The number of changes are greated then 2. Check 'git status'."$NC; return 1; }
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

commitDefaultsGoChanges() {
  set +e
  ask "3. Commit changes?"
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
  ask "4. Push image to quay.io?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    docker login quay.io -u $QUAY_USERNAME -p $QUAY_PASSWORD
    docker push quay.io/eclipse/che-operator:$RELEASE
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

updateNightlyOlmFiles() {
  set +e
  ask "5. Update nighlty OLM files?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    echo -e $GREEN"5.1 Launch 'update-nightly-olm-files.sh' script"$NC
    cd $BASE_DIR/olm
    . $BASE_DIR/olm/update-nightly-olm-files.sh
    cd $BASE_DIR

    echo -e $GREEN"5.2 Validate changes"$NC
    lastKubernetesNightlyDir=$(ls -dt $BASE_DIR/olm/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes/* | head -1)
    csvFile=$(ls ${lastKubernetesNightlyDir}/*.clusterserviceversion.yaml)
    checkImageReferences $csvFile

    lastNightlyOpenshiftDir=$(ls -dt $BASE_DIR/olm/eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift/* | head -1)
    csvFile=$(ls ${lastNightlyOpenshiftDir}/*.clusterserviceversion.yaml)
    checkImageReferences $csvFile

    echo -e $GREEN"5.3 It is needed to check file manully"$NC
    for diff in $(ls ${lastKubernetesNightlyDir}/*.diff); do echo $diff; done
    for diff in $(ls ${lastNightlyOpenshiftDir}/*.diff); do echo $diff; done

    echo -e $GREEN"5.4 Validate number of changed files"$NC
    local changes=$(git status -s | wc -l)
    [[ $changes -gt 4 ]] && { echo -e $RED"The number of changes are greated then 4. Check 'git status'."$NC; return 1; }

  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

commitNightlyOlmFiles() {
  set +e
  ask "6. Commit changes?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    git add -A
    git commit -m "Update nightly olm files" --signoff
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

releaseOlmFiles() {
  set +e
  ask "7. Release OLM files?"
  result=$?
  set -e

  if [[ $result == 0 ]]; then
    echo -e $GREEN"7.1 Launch 'olm/release-olm-files.sh' script"$NC
    cd $BASE_DIR/olm
    . $BASE_DIR/olm/release-olm-files.sh $RELEASE
    cd $CURRENT_DIR

    local openshift=$BASE_DIR/eclipse-che-preview-openshift/deploy/olm-catalog/eclipse-che-preview-openshift
    local kubernetes=$BASE_DIR/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes

    echo -e $GREEN"7.2 Validate files"$NC
    grep -q "currentCSV: eclipse-che-preview-openshift.v"$RELEASE $openshift/eclipse-che-preview-openshift.package.yaml
    grep -q "currentCSV: eclipse-che-preview-kubernetes.v"$RELEASE $kubernetes/eclipse-che-preview-kubernetes.package.yaml
    grep -q "version: "$RELEASE $openshift/$RELEASE/eclipse-che-preview-openshift.v$RELEASE.clusterserviceversion.yaml
    grep -q "version: "$RELEASE $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.v$RELEASE.clusterserviceversion.yaml
    test -f $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.crd.yaml
    test -f $openshift/$RELEASE/eclipse-che-preview-openshift.crd.yaml

    echo -e $GREEN"7.3 It is needed to check diff files manully"$NC
    echo $openshift/$RELEASE/eclipse-che-preview-openshift.v$RELEASE.clusterserviceversion.yaml.diff
    echo $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.v$RELEASE.clusterserviceversion.yaml.diff
    echo $openshift/$RELEASE/eclipse-che-preview-openshift.crd.yaml.diff
    echo $kubernetes/$RELEASE/eclipse-che-preview-kubernetes.crd.yaml.diff

    echo -e $GREEN"7.4 Validate number of changed files"$NC
    local changes=$(git status -s | wc -l)
    echo "Number for changes: "$changes
    [[ $changes -gt 4 ]] && { echo -e $RED"The number of changes is greater then 4. Check 'git status'."$NC; return 1; }
  elif [[ $result == 1 ]]; then
    echo -e $YELLOW"> SKIPPED"$NC
  fi
}

commitOlmChanges() {
  set +e
  ask "8. Commit changes?"
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
  ask "9. Push OLM files to quay.io?"
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
  ask "10. Push changes?"
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
  releaseOperatorCode
  commitDefaultsGoChanges
  pushImage
  updateNightlyOlmFiles
  commitNightlyOlmFiles
  releaseOlmFiles
  commitOlmChanges
  pushOlmFiles
  pushChanges
}

init "$@"
check "$@"
run "$@"
