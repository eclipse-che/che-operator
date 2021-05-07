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

TMP_JSON=$(mktemp)
cat "$OPERATOR_REPO"/.github/bin/resources/slack-message-template.json |
    sed -e "s#__REPLACE_DATE__#$(date)#g" |
    sed -e "s#__REPLACE_JOB_URL__#${BUILD_URL}#g" |
    sed -e "s#__REPLACE_JOB_RESULT__#${JOB_RESULT}#g" |
    cat >${TMP_JSON}

curl -X POST -d @${TMP_JSON} -H "Content-type:application/json; charset=utf-8" -X POST -H "Authorization: Bearer ${SLACK_TOKEN}" https://slack.com/api/chat.postMessage
