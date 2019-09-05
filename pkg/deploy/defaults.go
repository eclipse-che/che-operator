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

import (
	"strings"
)

const (
	defaultCheServerImageRepo           = "eclipse/che-server"
	defaultCodeReadyServerImageRepo     = "registry.redhat.io/codeready-workspaces/server-rhel8"
	defaultCheServerImageTag            = "7.1.0"
	defaultCodeReadyServerImageTag      = "2.0"
	DefaultCheFlavor                    = "che"
	DefaultChePostgresUser              = "pgche"
	DefaultChePostgresHostName          = "postgres"
	DefaultChePostgresPort              = "5432"
	DefaultChePostgresDb                = "dbche"
	DefaultPvcStrategy                  = "common"
	DefaultPvcClaimSize                 = "1Gi"
	DefaultIngressStrategy              = "multi-host"
	DefaultIngressClass                 = "nginx"
	defaultPluginRegistryImage          = "registry.redhat.io/codeready-workspaces/pluginregistry-rhel8:2.0"
	defaultPluginRegistryUpstreamImage  = "quay.io/eclipse/che-plugin-registry:7.1.0"
	DefaultPluginRegistryMemoryLimit    = "256Mi"
	DefaultPluginRegistryMemoryRequest  = "16Mi"
	defaultDevfileRegistryImage         = "registry.redhat.io/codeready-workspaces/devfileregistry-rhel8:2.0"
	defaultDevfileRegistryUpstreamImage = "quay.io/eclipse/che-devfile-registry:7.1.0"
	DefaultDevfileRegistryMemoryLimit   = "256Mi"
	DefaultDevfileRegistryMemoryRequest = "16Mi"
	DefaultKeycloakAdminUserName        = "admin"
	DefaultCheLogLevel                  = "INFO"
	DefaultCheDebug                     = "false"
	defaultPvcJobsImage                 = "registry.redhat.io/ubi8-minimal:8.0-159"
	defaultPvcJobsUpstreamImage         = "registry.access.redhat.com/ubi8-minimal:8.0-159"
	defaultPostgresImage                = "registry.redhat.io/rhscl/postgresql-96-rhel7:1-46"
	defaultPostgresUpstreamImage        = "centos/postgresql-96-centos7:9.6"
	defaultKeycloakImage                = "registry.redhat.io/redhat-sso-7/sso73-openshift:1.0-13"
	defaultKeycloakUpstreamImage        = "eclipse/che-keycloak:7.1.0"
	DefaultJavaOpts                     = "-XX:MaxRAMFraction=2 -XX:+UseParallelGC -XX:MinHeapFreeRatio=10 " +
		"-XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 " +
		"-XX:AdaptiveSizePolicyWeight=90 -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap " +
		"-Dsun.zip.disableMemoryMapping=true -Xms20m"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
	DefaultServerMemoryRequest          = "512Mi"
	DefaultServerMemoryLimit            = "1Gi"
	DefaultSecurityContextFsGroup       = "1724"
	DefaultSecurityContextRunAsUser     = "1724"

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next version
	// That fixed bug https://github.com/eclipse/che/issues/13714
	OldDefaultKeycloakUpstreamImageToDetect     = "eclipse/che-keycloak:7.0.0"
	OldDefaultPvcJobsUpstreamImageToDetect      = "registry.access.redhat.com/ubi8-minimal:8.0-127"
	OldDefaultPostgresUpstreamImageToDetect     = "centos/postgresql-96-centos7:9.6"
)

func DefaultCheServerImageTag(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultCodeReadyServerImageTag
	}
	return defaultCheServerImageTag
}

func DefaultCheServerImageRepo(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultCodeReadyServerImageRepo
	}
	return defaultCheServerImageRepo
}

func DefaultPvcJobsImage(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultPvcJobsImage
	}
	return defaultPvcJobsUpstreamImage
}

func DefaultPostgresImage(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultPostgresImage
	}
	return defaultPostgresUpstreamImage
}


func DefaultKeycloakImage(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultKeycloakImage
	}
	return defaultKeycloakUpstreamImage
}

func DefaultPluginRegistryImage(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultPluginRegistryImage
	}
	return defaultPluginRegistryUpstreamImage
}

func DefaultDevfileRegistryImage(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultDevfileRegistryImage
	}
	return defaultDevfileRegistryUpstreamImage
}

func DefaultPullPolicyFromDockerImage(dockerImage string) string {
	tag := "latest"
	parts := strings.Split(dockerImage, ":")
	if len(parts) > 1 {
		tag = parts[1]
	}
	if tag == "latest" || tag == "nightly" {
		return "Always"
	}
	return "IfNotPresent"
}
