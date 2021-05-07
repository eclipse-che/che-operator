#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u

export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
export HAPPY_PATH_POD_NAME=happy-path-che
export HAPPY_PATH_DEVFILE='https://gist.githubusercontent.com/l0rd/71a04dd0d8c8e921b16ba2690f7d5a47/raw/d520086e148c359b18c229328824dfefcf85e5ef/spring-petclinic-devfile-v2.0.0.yaml'
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}"/.github/bin/oauth-provision.sh

# Stop execution on any error
trap "Catch_Finish" EXIT SIGINT

function bumpPodsInfo() {
    NS=$1
    TARGET_DIR="${ARTIFACTS_DIR}/${NS}-info"
    mkdir -p "$TARGET_DIR"

    for POD in $(oc get pods -o name -n ${NS}); do
        for CONTAINER in $(oc get -n ${NS} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
            echo ""
            echo "======== Getting logs from container $POD/$CONTAINER in $NS"
            echo ""
            # container name includes `pod/` prefix. remove it
            LOGS_FILE=$TARGET_DIR/$(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
            oc logs ${POD} -c ${CONTAINER} -n ${NS} > $LOGS_FILE || true
        done
    done
    echo "======== Bumping events -n ${NS} ========"
    oc get events -n $NS -o=yaml > $TARGET_DIR/events.log || true
}

function Catch_Finish() {
    # grab devworkspace-controller namespace events after running e2e
    bumpPodsInfo "devworkspace-controller"
    bumpPodsInfo "admin-che"
    oc get devworkspaces -n "admin-che" -o=yaml > $ARTIFACTS_DIR/devworkspaces.yaml

    collectCheLogWithChectl
}

overrideDefaults() {
  # CI_CHE_OPERATOR_IMAGE it is che operator image builded in openshift CI job workflow. More info about how works image dependencies in ci:https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
  export OPERATOR_IMAGE=${CI_CHE_OPERATOR_IMAGE:-"quay.io/eclipse/che-operator:nightly"}
  echo ${OPERATOR_IMAGE}
}

deployChe() {
  cat > /tmp/che-cr-patch.yaml <<EOL
spec:
  devWorkspace:
    enable: true
  server:
    customCheProperties:
      CHE_FACTORY_DEFAULT__PLUGINS: ""
      CHE_WORKSPACE_DEVFILE_DEFAULT__EDITOR_PLUGINS: ""
  auth:
    updateAdminPassword: false
EOL

  cat /tmp/che-cr-patch.yaml

  chectl server:deploy --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml -p openshift --templates=${TEMPLATES} --batch --telemetry=off --installer=operator --che-operator-image=${OPERATOR_IMAGE}
}

startHappyPathTest() {
  # patch happy-path-che.yaml
  ECLIPSE_CHE_URL=http://$(oc get route -n "${NAMESPACE}" che -o jsonpath='{.status.ingress[0].host}')
  TS_SELENIUM_DEVWORKSPACE_URL="${ECLIPSE_CHE_URL}/#${HAPPY_PATH_DEVFILE}"
  sed -i "s@CHE_URL@${ECLIPSE_CHE_URL}@g" ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  sed -i "s@WORKSPACE_ROUTE@${TS_SELENIUM_DEVWORKSPACE_URL}@g" ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  sed -i "s@CHE-NAMESPACE@${NAMESPACE}@g" ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  cat ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml

  oc apply -f ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  # wait for the pod to start
  n=0
  while [ $n -le 120 ]
  do
    PHASE=$(oc get pod -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} \
        --template='{{ .status.phase }}')
    if [[ ${PHASE} == "Running" ]]; then
      echo "[INFO] Happy-path test started succesfully."
      return
    fi

    sleep 5
    n=$(( n+1 ))
  done

  echo "[ERROR] Failed to start happy-path test."
  exit 1
}

runTest() {
  deployChe

  startHappyPathTest

  # wait for the test to finish
  oc logs -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c happy-path-test -f

  # just to sleep
  sleep 3

  # download the test results
  mkdir -p /tmp/e2e
  oc rsync -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME}:/tmp/e2e/report/ /tmp/e2e -c download-reports
  oc exec -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c download-reports -- touch /tmp/done

  mkdir -p ${ARTIFACTS_DIR}
  cp -r /tmp/e2e ${ARTIFACTS_DIR}

  EXIT_CODE=$(oc logs -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c happy-path-test | grep EXIT_CODE)

  if [[ ${EXIT_CODE} == "+ EXIT_CODE=1" ]]; then
    echo "[ERROR] Happy-path test failed."
    exit 1
  fi
}

initDefaults
initLatestTemplates
overrideDefaults
provisionOpenShiftOAuthUser
runTest
