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
package deploy

const (
	DefaultCheServerImageRepo       = "eclipse/che-server"
	DefaultCodeReadyServerImageRepo = "registry.access.redhat.com/codeready-workspaces/server"
	DefaultCheServerImageTag        = "7.0.0-beta-2.0"
	DefaultCodeReadyServerImageTag  = "1.1"
	DefaultCheFlavor                = "che"
	DefaultChePostgresUser          = "pgche"
	DefaultChePostgresHostName      = "postgres"
	DefaultChePostgresPort          = "5432"
	DefaultChePostgresDb            = "dbche"
	DefaultPvcStrategy              = "common"
	DefaultPvcClaimSize             = "1Gi"
	DefaultIngressStrategy          = "multi-host"
	DefaultIngressClass             = "nginx"
	DefaultPluginRegistryUrl        = "https://che-plugin-registry.openshift.io"
	DefaultKeycloakAdminUserName    = "admin"
	DefaultCheLogLevel              = "INFO"
	DefaultCheDebug                 = "false"
	DefaultPvcJobsImage             = "registry.access.redhat.com/rhel7-minimal:7.6-154"
	DefaultPostgresImage            = "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1-25"
	DefaultPostgresUpstreamImage    = "centos/postgresql-96-centos7:9.6"
	DefaultKeycloakImage            = "registry.access.redhat.com/redhat-sso-7/sso72-openshift:1.2-8"
	DefaultKeycloakUpstreamImage    = "eclipse/che-keycloak:6.19.0"
	DefaultJavaOpts                 = "-XX:MaxRAMFraction=2 -XX:+UseParallelGC -XX:MinHeapFreeRatio=10 " +
		"-XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 " +
		"-XX:AdaptiveSizePolicyWeight=90 -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap " +
		"-Dsun.zip.disableMemoryMapping=true -Xms20m"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
)
