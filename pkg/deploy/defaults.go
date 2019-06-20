//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
// REMINDER: when updating versions below, see also pkg/apis/org/v1/che_types.go and deploy/crds/org_v1_che_cr.yaml
package deploy

const (
	DefaultCheServerImageRepo        = "eclipse/che-server"
	DefaultCodeReadyServerImageRepo  = "registry.redhat.io/codeready-workspaces/server-rhel8"
	DefaultCheServerImageTag         = "7.0.0-RC-2.0"
	DefaultCodeReadyServerImageTag   = "1.2"
	DefaultCheFlavor                 = "che"
	DefaultChePostgresUser           = "pgche"
	DefaultChePostgresHostName       = "postgres"
	DefaultChePostgresPort           = "5432"
	DefaultChePostgresDb             = "dbche"
	DefaultPvcStrategy               = "common"
	DefaultPvcClaimSize              = "1Gi"
	DefaultIngressStrategy           = "multi-host"
	DefaultIngressClass              = "nginx"
	DefaultPluginRegistryUrl         = "https://che-plugin-registry.openshift.io"
	DefaultUpstreamPluginRegistryUrl = "https://che-plugin-registry.openshift.io/v3"
	DefaultKeycloakAdminUserName     = "admin"
	DefaultCheLogLevel               = "INFO"
	DefaultCheDebug                  = "false"
	DefaultPvcJobsImage              = "registry.redhat.io/ubi8-minimal:8.0-127"
	DefaultPvcJobsUpstreamImage      = "registry.access.redhat.com/ubi8-minimal:8.0-127"
	DefaultPostgresImage             = "registry.redhat.io/rhscl/postgresql-96-rhel7:1-40"
	DefaultPostgresUpstreamImage     = "centos/postgresql-96-centos7:9.6"
	DefaultKeycloakImage             = "registry.redhat.io/redhat-sso-7/sso73-openshift:1.0-11"
	DefaultKeycloakUpstreamImage     = "eclipse/che-keycloak:7.0.0-RC-2.0"
	DefaultJavaOpts                  = "-XX:MaxRAMFraction=2 -XX:+UseParallelGC -XX:MinHeapFreeRatio=10 " +
		"-XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 " +
		"-XX:AdaptiveSizePolicyWeight=90 -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap " +
		"-Dsun.zip.disableMemoryMapping=true -Xms20m"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
	DefaultServerMemoryRequest = "512Mi"
	DefaultServerMemoryLimit   = "1Gi"
	DefaultSecurityContextFsGroup    = "1724"
	DefaultSecurityContextRunAsUser  = "1724"
)
