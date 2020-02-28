#!/bin/bash

# todo add ability configure namespace
# check if dlv is preinstaled, otherwise throw and error about required depenedency

export WATCH_NAMESPACE=che

operator-sdk up local --namespace=che --enable-delve --operator-flags "--flag1 value1 --flag2=value2"

# --operator-flags "--defaultsPath \"/home/user/GoWorkSpace/src/github.com/eclipse/che-operator/deploy/operator.yaml\""