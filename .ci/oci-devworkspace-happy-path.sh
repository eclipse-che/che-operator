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
export HAPPY_PATH_DEVFILE='https://github.com/l0rd/spring-petclinic/tree/devfile2'
source "${OPERATOR_REPO}"/.github/bin/common.sh
source "${OPERATOR_REPO}"/.github/bin/oauth-provision.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

deployChe() {
  cat > /tmp/che-cr-patch.yaml <<EOL
spec:
  devWorkspace:
    enable: true
  server:
    customCheProperties:
      CHE_FACTORY_DEFAULT__PLUGINS: ""
      CHE_WORKSPACE_DEVFILE_DEFAULT__EDITOR_PLUGINS: ""
EOL

  cat /tmp/che-cr-patch.yaml

  chectl server:deploy --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml -p openshift --batch --telemetry=off --installer=operator
}

startHappyPathTest() {
  # patch happy-path-che.yaml 
  ECLIPSE_CHE_URL=http://$(oc get route -n "${NAMESPACE}" che -o jsonpath='{.status.ingress[0].host}')
  TS_SELENIUM_DEVWORKSPACE_URL="${ECLIPSE_CHE_URL}/#${HAPPY_PATH_DEVFILE}"
  sed -i "s@CHE_URL@${ECLIPSE_CHE_URL}@g" ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  sed -i "s@WORKSPACE_ROUTE@${TS_SELENIUM_DEVWORKSPACE_URL}@g" ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  cat ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml

  oc apply -f ${OPERATOR_REPO}/.ci/openshift-ci/happy-path-che.yaml
  # wait for the pod to start
  n=0
  while [ $n -le 120 ]
  do
    PHASE=$(oc get pod -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} \
        --template='{{ .status.phase }}')
    if [[ ${PHASE} == "Running" ]]; then
      echo "[INFO] Happy-path test started succesfully"
      return
    fi

    sleep 5
    n=$(( n+1 ))
  done

  echo "Failed to start happy-path test"
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
}

initDefaults
provisionOpenShiftOAuthUser
runTest
