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

set -e

curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.10.0/install.sh | bash -s 0.10.0
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/01_namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/02_catalogsourceconfig.crd.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/03_operatorsource.crd.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/04_service_account.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/05_role.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/06_role_binding.yaml
sleep 1
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/07_upstream_operatorsource.cr.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/08_operator.yaml
kubectl apply -f operator-source.yaml

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
---
apiVersion: "operators.coreos.com/v1"
kind: "CatalogSourceConfig"
metadata:
  name: "installed-eclipse-che-preview"
  namespace: "marketplace"
spec:
  targetNamespace: eclipse-che-preview-test
  source: eclipse-che-preview-kubernetes
  packages: eclipse-che-preview-kubernetes
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: eclipse-che-preview-test
  namespace: eclipse-che-preview-test
spec:
  channel: nightly
  name: eclipse-che-preview-kubernetes
  source: installed-eclipse-che-preview
  sourceNamespace: eclipse-che-preview-test
---
apiVersion: org.eclipse.che/v1
kind: CheCluster
metadata:
  name: eclipse-che
  namespace: eclipse-che-preview-test
spec:
  k8s:
    ingressDomain: '$(minikube ip).nip.io'
    tlsSecretName: ''
  server:
    cheImageTag: 'nightly'
    devfileRegistryImage: 'quay.io/eclipse/che-devfile-registry:nightly'
    pluginRegistryImage: 'quay.io/eclipse/che-plugin-registry:nightly'
    tlsSupport: false
    selfSignedCert: false
  database:
    externalDb: false
    chePostgresHostname: ''
    chePostgresPort: ''
    chePostgresUser: ''
    chePostgresPassword: ''
    chePostgresDb: ''
  auth:
    identityProviderImage: 'eclipse/che-keycloak:nightly'
    externalIdentityProvider: false
    identityProviderURL: ''
    identityProviderRealm: ''
    identityProviderClientId: ''
  storage:
    pvcStrategy: per-workspace
    pvcClaimSize: 1Gi
    preCreateSubPaths: true
EOF
