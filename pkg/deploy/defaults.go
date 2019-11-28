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
	"fmt"
	"strings"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

const (
	defaultCheServerImageRepo           = "eclipse/che-server"
	defaultCodeReadyServerImageRepo     = "registry.redhat.io/codeready-workspaces/server-rhel8"
	defaultCheServerImageTag            = "7.4.0"
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
	defaultPluginRegistryUpstreamImage  = "quay.io/eclipse/che-plugin-registry:7.4.0"
	DefaultPluginRegistryMemoryLimit    = "256Mi"
	DefaultPluginRegistryMemoryRequest  = "16Mi"
	defaultDevfileRegistryImage         = "registry.redhat.io/codeready-workspaces/devfileregistry-rhel8:2.0"
	defaultDevfileRegistryUpstreamImage = "quay.io/eclipse/che-devfile-registry:7.4.0"
	DefaultDevfileRegistryMemoryLimit   = "256Mi"
	DefaultDevfileRegistryMemoryRequest = "16Mi"
	DefaultKeycloakAdminUserName        = "admin"
	DefaultCheLogLevel                  = "INFO"
	DefaultCheDebug                     = "false"
	DefaultCheMetricsPort               = int32(8087)
	defaultPvcJobsImage                 = "registry.redhat.io/ubi8-minimal:8.0-213"
	defaultPvcJobsUpstreamImage         = "registry.access.redhat.com/ubi8-minimal:8.0-213"
	defaultPostgresImage                = "registry.redhat.io/rhscl/postgresql-96-rhel7:1-47"
	defaultPostgresUpstreamImage        = "centos/postgresql-96-centos7:9.6"
	defaultKeycloakImage                = "registry.redhat.io/redhat-sso-7/sso73-openshift:1.0-15"
	defaultKeycloakUpstreamImage        = "eclipse/che-keycloak:7.4.0"
	DefaultJavaOpts                     = "-XX:MaxRAMFraction=2 -XX:+UseParallelGC -XX:MinHeapFreeRatio=10 " +
		"-XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 " +
		"-XX:AdaptiveSizePolicyWeight=90 -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap " +
		"-Dsun.zip.disableMemoryMapping=true -Xms20m"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
	DefaultServerMemoryRequest      = "512Mi"
	DefaultServerMemoryLimit        = "1Gi"
	DefaultSecurityContextFsGroup   = "1724"
	DefaultSecurityContextRunAsUser = "1724"

	// CRW images for that are mentioned in the Che server che.properties
	// For CRW these should be synced by hand with images stored in RH registries
	// instead of being synced by script with the content of the upstream `che.properties` file
	// NB:
	// The upstream equivent are stored in the generated `extra_images.go` source file.
	defaultCheWorkspacePluginBrokerInitImage    = "registry.redhat.io/codeready-workspaces/pluginbrokerinit-rhel8:2.0"
	defaultCheWorkspacePluginBrokerUnifiedImage = "registry.redhat.io/codeready-workspaces/pluginbroker-rhel8:2.0"
	defaultCheServerSecureExposerJwtProxyImage  = "registry.redhat.io/codeready-workspaces/jwtproxy-rhel8:2.0"

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next version
	// That fixed bug https://github.com/eclipse/che/issues/13714
	OldDefaultKeycloakUpstreamImageToDetect = "eclipse/che-keycloak:7.0.0"
	OldDefaultPvcJobsUpstreamImageToDetect  = "registry.access.redhat.com/ubi8-minimal:8.0-127"
	OldDefaultPostgresUpstreamImageToDetect = "centos/postgresql-96-centos7:9.6"

	OldDefaultCodeReadyServerImageRepo = "registry.redhat.io/codeready-workspaces/server-rhel8"
	OldDefaultCodeReadyServerImageTag  = "1.2"
	OldCrwPluginRegistryUrl            = "https://che-plugin-registry.openshift.io"

	// ConsoleLink default
	DefaultConsoleLinkName                = "che"
	DefaultConsoleLinkSection             = "Red Hat Applications"
	DefaultConsoleLinkImage               = "/dashboard/assets/branding/loader.svg"
	defaultConsoleLinkUpstreamDisplayName = "Eclipse Che"
	defaultConsoleLinkDisplayName         = "CodeReady Workspaces"
)

func MigratingToCRW2_0(cr *orgv1.CheCluster) bool {
	if cr.Spec.Server.CheFlavor == "codeready" &&
		strings.HasPrefix(cr.Status.CheVersion, "1.2") &&
		strings.HasPrefix(defaultCodeReadyServerImageTag, "2.0") {
		return true
	}
	return false
}

func DefaultConsoleLinkDisplayName(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultConsoleLinkDisplayName
	}
	return defaultConsoleLinkUpstreamDisplayName
}

func DefaultCheServerImageTag(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultCodeReadyServerImageTag
	}
	return defaultCheServerImageTag
}

func DefaultCheServerImageRepo(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		return patchDefaultImageName(cr, defaultCodeReadyServerImageRepo)
	} else {
		return patchDefaultImageName(cr, defaultCheServerImageRepo)
	}
}

func DefaultPvcJobsImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		return patchDefaultImageName(cr, defaultPvcJobsImage)
	}
	return patchDefaultImageName(cr, defaultPvcJobsUpstreamImage)
}

func DefaultPostgresImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		return patchDefaultImageName(cr, defaultPostgresImage)
	} else {
		return patchDefaultImageName(cr, defaultPostgresUpstreamImage)
	}
}

func DefaultKeycloakImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		return patchDefaultImageName(cr, defaultKeycloakImage)
	} else {
		return patchDefaultImageName(cr, defaultKeycloakUpstreamImage)
	}
}

func DefaultPluginRegistryImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		return patchDefaultImageName(cr, defaultPluginRegistryImage)
	} else {
		return patchDefaultImageName(cr, defaultPluginRegistryUpstreamImage)
	}
}

func DefaultDevfileRegistryImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		return patchDefaultImageName(cr, defaultDevfileRegistryImage)
	} else {
		return patchDefaultImageName(cr, defaultDevfileRegistryUpstreamImage)
	}
}

func DefaultCheWorkspacePluginBrokerInitImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		// In the CRW case, we should always set the plugin broker image in the Che config map
		return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerInitImage)
	} else {
		// In the Upstream Che case, the default will be provided by the Che server `che.properties` file
		// if we return an empty string here.
		// We only need to override it in case of AirGap mode
		if cr.IsAirGapMode() {
			return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerInitUpstreamImage)
		}
		return ""
	}
}

func DefaultCheWorkspacePluginBrokerUnifiedImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		// In the CRW case, we should always set the plugin broker image in the Che config map
		return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerUnifiedImage)
	} else {
		// In the Upstream Che case, the default will be provided by the Che server `che.properties` file
		// if we return an empty string here.
		// We only need to override it in case of AirGap mode
		if cr.IsAirGapMode() {
			return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerUnifiedUpstreamImage)
		}
		return ""
	}
}

func DefaultCheServerSecureExposerJwtProxyImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		// In the CRW case, we should always set the jwt-proxy image in the Che config map
		return patchDefaultImageName(cr, defaultCheServerSecureExposerJwtProxyImage)
	} else {
		// In the Upstream Che case, the default will be provided by the Che server `che.properties` file
		// if we return an empty string here.
		// We only need to override it in case of AirGap mode
		if cr.IsAirGapMode() {
			return patchDefaultImageName(cr, defaultCheServerSecureExposerJwtProxyUpstreamImage)
		}
		return ""
	}
}

func DefaultCheWorkspacePluginBrokerInitImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		// In the CRW case, we should always set the plugin broker image in the Che config map
		return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerInitImage)
	} else {
		// In the Upstream Che case, the default will be provided by the Che server `che.properties` file
		// if we return an empty string here.
		// We only need to override it in case of AirGap mode
		if cr.IsAirGapMode() {
			return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerInitUpstreamImage)
		}
		return ""
	}
}

func DefaultCheWorkspacePluginBrokerUnifiedImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		// In the CRW case, we should always set the plugin broker image in the Che config map
		return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerUnifiedImage)
	} else {
		// In the Upstream Che case, the default will be provided by the Che server `che.properties` file
		// if we return an empty string here.
		// We only need to override it in case of AirGap mode
		if cr.IsAirGapMode() {
			return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerUnifiedUpstreamImage)
		}
		return ""
	}
}

func DefaultCheServerSecureExposerJwtProxyImage(cr *orgv1.CheCluster, cheFlavor string) string {
	if cheFlavor == "codeready" {
		// In the CRW case, we should always set the jwt-proxy image in the Che config map
		return patchDefaultImageName(cr, defaultCheServerSecureExposerJwtProxyImage)
	} else {
		// In the Upstream Che case, the default will be provided by the Che server `che.properties` file
		// if we return an empty string here.
		// We only need to override it in case of AirGap mode
		if cr.IsAirGapMode() {
			return patchDefaultImageName(cr, defaultCheServerSecureExposerJwtProxyUpstreamImage)
		}
		return ""
	}
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

func patchDefaultImageName(cr *orgv1.CheCluster, imageName string) string {
	if !cr.IsAirGapMode() {
		return imageName
	}
	var hostname, organization string
	if cr.Spec.Server.AirGapContainerRegistryHostname != "" {
		hostname = cr.Spec.Server.AirGapContainerRegistryHostname
	} else {
		hostname = getHostnameFromImage(imageName)
	}
	if cr.Spec.Server.AirGapContainerRegistryOrganization != "" {
		organization = cr.Spec.Server.AirGapContainerRegistryOrganization
	} else {
		organization = getOrganizationFromImage(imageName)
	}
	image := getImageNameFromFullImage(imageName)
	return fmt.Sprintf("%s/%s/%s", hostname, organization, image)
}

func getImageNameFromFullImage(image string) string {
	imageParts := strings.Split(image, "/")
	nameAndTag := ""
	switch len(imageParts) {
	case 1:
		nameAndTag = imageParts[0]
	case 2:
		nameAndTag = imageParts[1]
	case 3:
		nameAndTag = imageParts[2]
	}
	return nameAndTag
}

func getHostnameFromImage(image string) string {
	imageParts := strings.Split(image, "/")
	hostname := ""
	switch len(imageParts) {
	case 3:
		hostname = imageParts[0]
	default:
		hostname = "docker.io"
	}
	return hostname
}

func getOrganizationFromImage(image string) string {
	imageParts := strings.Split(image, "/")
	organization := ""
	switch len(imageParts) {
	case 2:
		organization = imageParts[0]
	case 3:
		organization = imageParts[1]
	}
	return organization
}
