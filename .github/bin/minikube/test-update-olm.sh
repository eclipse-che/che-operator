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

set -e

export OPERATOR_REPO=$(dirname $(dirname $(dirname $(dirname $(readlink -f "${BASH_SOURCE[0]}")))))
NAMESPACE="eclipse-che"
export CTSRC_IMAGE="myimage:latest"
export CONTAINER_HOST=ssh://root@linuxhost/run/podman/podman.sock


#"imagePullPolicy":"IfNotPresent"
source "${OPERATOR_REPO}"/.github/bin/common.sh

# Stop execution on any error
trap "catchFinish" EXIT SIGINT

insecurePrivateDockerRegistry

# Build latest stable catalog image
buildK8SCheCatalogImage() {
  echo ${OPERATOR_REPO}
  eval $(minikube podman-env)
  podman build -t "${CTSRC_IMAGE}" -f "${OPERATOR_REPO}"/olm/eclipse-che-preview-kubernetes/Dockerfile ${OPERATOR_REPO}/olm/eclipse-che-preview-kubernetes
  printf '%s\t%s\n' "$(minikube ip)" 'registry.kube' | sudo tee -a /etc/hosts

  podman push $CTSRC_IMAGE $(minikube ip):5000/$CTSRC_IMAGE
  exit 0
}

buildK8SCheCatalogImage

CATSRC=$(
    oc create -f - -o jsonpath='{.metadata.name}' <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: catalog-operator
  namespace: eclipse-che
spec:
  sourceType: grpc
  image: localhost/"$CTSRC_IMAGE"
EOF
)
OPERATOR_POD=$(kubectl get pods -o json -n ${NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("catalog-operator-")).metadata.name')
exit 0
oc patch pod "$OPERATOR_POD" -n ${NAMESPACE} --type='json' -p='[{"op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value":"IfNotPresent"}]'

echo "CatalogSource name is \"$CATSRC\""
