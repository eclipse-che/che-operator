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
  namespace: "openshift-marketplace"
spec:
  targetNamespace: eclipse-che-preview-test
  source: eclipse-che-preview-openshift
  packages: eclipse-che-preview-openshift
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: eclipse-che-preview-test
  namespace: eclipse-che-preview-test
spec:
  channel: nightly
  name: eclipse-che-preview-openshift
  source: installed-eclipse-che-openshift
  sourceNamespace: eclipse-che-preview-test
---
apiVersion: org.eclipse.che/v1
kind: CheCluster
metadata:
  name: eclipse-che
  namespace: eclipse-che-preview-test
spec:
  server:
    cheImageTag: nightly
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
    openShiftoAuth: true
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
